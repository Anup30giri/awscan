package aws

import (
	"testing"

	appconfig "github.com/anupgiri/awscan/internal/config"
)

func TestResolveProfileNamePrecedence(t *testing.T) {
	t.Parallel()

	t.Setenv("AWS_PROFILE", "env")

	resolver := &ConfigResolver{
		preferences: &appconfig.Preferences{DefaultProfile: "config"},
	}

	profile, source := resolver.resolveProfileName(ResolveOptions{Profile: "flag"}, nil)
	if profile != "flag" || source != "flag" {
		t.Fatalf("resolveProfileName() = (%q, %q), want (flag, flag)", profile, source)
	}
}

func TestResolveRegionPrecedence(t *testing.T) {
	t.Parallel()

	t.Setenv("AWS_REGION", "us-west-2")

	resolver := &ConfigResolver{
		preferences: &appconfig.Preferences{DefaultRegion: "ap-south-1"},
	}

	region := resolver.resolveRegion(ResolveOptions{}, "", nil)
	if region != "us-west-2" {
		t.Fatalf("resolveRegion() = %q, want us-west-2", region)
	}
}
