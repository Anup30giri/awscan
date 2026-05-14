package aws

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"

	appconfig "github.com/anupgiri/awscan/internal/config"
)

type ConfigResolver struct {
	sharedPaths SharedConfigPaths
	preferences *appconfig.Preferences
}

func NewConfigResolver(preferences *appconfig.Preferences) *ConfigResolver {
	return &ConfigResolver{
		sharedPaths: DefaultSharedConfigPaths(),
		preferences: preferences,
	}
}

func (r *ConfigResolver) Resolve(ctx context.Context, opts ResolveOptions) (Runtime, error) {
	profiles, err := LoadProfiles(r.sharedPaths)
	if err != nil {
		return Runtime{}, err
	}

	profileName, source := r.resolveProfileName(opts, profiles)
	region := r.resolveRegion(opts, profileName, profiles)
	profile, profileFound := FindProfile(profiles, profileName)
	kind := ProfileKindEnvironment
	if profileFound {
		kind = profile.Kind
	}
	if profileName == "" {
		kind = ProfileKindEnvironment
	}

	loadOptions := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithEC2IMDSClientEnableState(imds.ClientDisabled),
		awsconfig.WithRegion(region),
	}

	if profileName != "" {
		loadOptions = append(loadOptions, awsconfig.WithSharedConfigProfile(profileName))
	}

	if kind == ProfileKindLoginSession {
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(
			sdkaws.NewCredentialsCache(NewExportedCredentialProvider(profileName)),
		))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return Runtime{}, r.wrapCredentialError(profileName, region, err)
	}

	if _, err := cfg.Credentials.Retrieve(ctx); err != nil {
		return Runtime{}, r.wrapCredentialError(profileName, region, err)
	}

	return Runtime{
		Config:      cfg,
		Profile:     profileName,
		Region:      region,
		ProfileKind: kind,
		Available:   profiles,
		ResolvedAt:  time.Now().UTC(),
		Source:      source,
	}, nil
}

func (r *ConfigResolver) resolveProfileName(opts ResolveOptions, profiles []Profile) (string, string) {
	if opts.Profile != "" {
		return opts.Profile, "flag"
	}
	if value := os.Getenv("AWS_PROFILE"); value != "" {
		return value, "env"
	}
	if r.preferences != nil && r.preferences.DefaultProfile != "" {
		return r.preferences.DefaultProfile, "config"
	}
	if _, ok := FindProfile(profiles, defaultProfileName); ok {
		return defaultProfileName, "aws-config"
	}
	return "", "environment"
}

func (r *ConfigResolver) resolveRegion(opts ResolveOptions, profileName string, profiles []Profile) string {
	if opts.Region != "" {
		return opts.Region
	}
	if value := os.Getenv("AWS_REGION"); value != "" {
		return value
	}
	if value := os.Getenv("AWS_DEFAULT_REGION"); value != "" {
		return value
	}
	if r.preferences != nil && r.preferences.DefaultRegion != "" {
		return r.preferences.DefaultRegion
	}
	if profile, ok := FindProfile(profiles, profileName); ok && profile.Region != "" {
		return profile.Region
	}
	return "us-east-1"
}

func (r *ConfigResolver) wrapCredentialError(profile, region string, err error) error {
	message := err.Error()
	switch {
	case strings.Contains(message, "EC2 IMDS access disabled"):
		return fmt.Errorf("could not find local AWS credentials for profile %q in region %q. This machine does not appear to be an EC2 instance. Run `aws sts get-caller-identity` to verify your login", profile, region)
	case strings.Contains(message, "failed to refresh cached credentials"):
		return fmt.Errorf("could not load AWS credentials for profile %q in region %q: %w", profile, region, err)
	default:
		return fmt.Errorf("aws credential resolution failed for profile %q in region %q: %w", profile, region, err)
	}
}

func DetectEnvironmentCredentials() bool {
	return os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != ""
}

func IsLoginSessionProfile(profile Profile) bool {
	return profile.Kind == ProfileKindLoginSession || profile.Properties["login_session"] != ""
}

func ValidateRegion(region string) error {
	if strings.TrimSpace(region) == "" {
		return errors.New("region cannot be empty")
	}
	return nil
}
