package aws

import (
	"context"
	"time"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
)

type ProfileKind string

const (
	ProfileKindUnknown           ProfileKind = "unknown"
	ProfileKindStandard          ProfileKind = "standard"
	ProfileKindAssumeRole        ProfileKind = "assume-role"
	ProfileKindSSO               ProfileKind = "sso"
	ProfileKindCredentialProcess ProfileKind = "credential_process"
	ProfileKindLoginSession      ProfileKind = "login_session"
	ProfileKindEnvironment       ProfileKind = "environment"
)

type ResolveOptions struct {
	Profile string
	Region  string
}

type Runtime struct {
	Config         sdkaws.Config
	Profile        string
	Region         string
	ProfileKind    ProfileKind
	Available      []Profile
	Identity       *Identity
	ResolvedAt     time.Time
	Source         string
}

type Identity struct {
	Account string
	ARN     string
	UserID  string
}

type Profile struct {
	Name       string
	Properties map[string]string
	Kind       ProfileKind
	Region     string
}

type Resolver interface {
	Resolve(ctx context.Context, opts ResolveOptions) (Runtime, error)
}
