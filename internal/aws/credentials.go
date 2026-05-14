package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
)

type ExportedCredentialProvider struct {
	profile string

	mu     sync.Mutex
	cached sdkaws.Credentials
}

type processCredentialDocument struct {
	Version         int    `json:"Version"`
	AccessKeyID     string `json:"AccessKeyId"`
	SecretAccessKey string `json:"SecretAccessKey"`
	SessionToken    string `json:"SessionToken"`
	Expiration      string `json:"Expiration"`
}

func NewExportedCredentialProvider(profile string) *ExportedCredentialProvider {
	return &ExportedCredentialProvider{profile: profile}
}

func (p *ExportedCredentialProvider) Retrieve(ctx context.Context) (sdkaws.Credentials, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cached.HasKeys() && (!p.cached.CanExpire || time.Until(p.cached.Expires) > 2*time.Minute) {
		return p.cached, nil
	}

	creds, err := p.retrieveFresh(ctx)
	if err != nil {
		return sdkaws.Credentials{}, err
	}

	p.cached = creds
	return creds, nil
}

func (p *ExportedCredentialProvider) retrieveFresh(ctx context.Context) (sdkaws.Credentials, error) {
	if p.profile == "" {
		return sdkaws.Credentials{}, errors.New("missing profile for login_session credential export")
	}

	cmd := exec.CommandContext(ctx, "aws", "configure", "export-credentials", "--profile", p.profile, "--format", "process")
	output, err := cmd.Output()
	if err != nil {
		return sdkaws.Credentials{}, fmt.Errorf("refresh credentials for login_session profile %q: %w. Run `aws login --profile %s` or `aws sso login --profile %s` and try again", p.profile, err, p.profile, p.profile)
	}

	doc, err := ParseProcessCredentialDocument(output)
	if err != nil {
		return sdkaws.Credentials{}, err
	}

	expires := time.Time{}
	canExpire := false
	if doc.Expiration != "" {
		parsed, parseErr := time.Parse(time.RFC3339, doc.Expiration)
		if parseErr != nil {
			return sdkaws.Credentials{}, fmt.Errorf("parse exported credential expiration: %w", parseErr)
		}
		expires = parsed
		canExpire = true
	}

	return sdkaws.Credentials{
		AccessKeyID:     doc.AccessKeyID,
		SecretAccessKey: doc.SecretAccessKey,
		SessionToken:    doc.SessionToken,
		CanExpire:       canExpire,
		Expires:         expires,
		Source:          "aws-configure-export-credentials",
	}, nil
}

func ParseProcessCredentialDocument(data []byte) (*processCredentialDocument, error) {
	var doc processCredentialDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse credential_process json: %w", err)
	}

	if doc.Version != 1 {
		return nil, fmt.Errorf("unsupported credential_process version %d", doc.Version)
	}
	if doc.AccessKeyID == "" || doc.SecretAccessKey == "" {
		return nil, errors.New("credential_process output is missing access key material")
	}

	return &doc, nil
}
