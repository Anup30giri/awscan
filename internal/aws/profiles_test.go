package aws

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProfilesAndClassification(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config")
	credentialsPath := filepath.Join(dir, "credentials")

	config := `
[default]
region = ap-south-1
login_session = arn:aws:sts::123:assumed-role/demo/user

[profile proc]
region = us-east-1
credential_process = aws configure export-credentials --profile proc --format process

[profile assumed]
role_arn = arn:aws:iam::123:role/demo
source_profile = default

[profile sso]
sso_session = corp
region = eu-west-1
`
	creds := `
[static]
aws_access_key_id = foo
aws_secret_access_key = bar
`

	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(credentialsPath, []byte(creds), 0o600); err != nil {
		t.Fatal(err)
	}

	profiles, err := LoadProfiles(SharedConfigPaths{
		ConfigFile:      configPath,
		CredentialsFile: credentialsPath,
	})
	if err != nil {
		t.Fatalf("LoadProfiles() error = %v", err)
	}

	assertProfileKind(t, profiles, "default", ProfileKindLoginSession)
	assertProfileKind(t, profiles, "proc", ProfileKindCredentialProcess)
	assertProfileKind(t, profiles, "assumed", ProfileKindAssumeRole)
	assertProfileKind(t, profiles, "sso", ProfileKindSSO)
	assertProfileKind(t, profiles, "static", ProfileKindStandard)
}

func TestIsLoginSessionProfile(t *testing.T) {
	t.Parallel()

	profile := Profile{
		Name: "default",
		Properties: map[string]string{
			"login_session": "arn:aws:sts::123:assumed-role/demo/user",
		},
		Kind: ProfileKindLoginSession,
	}

	if !IsLoginSessionProfile(profile) {
		t.Fatal("expected login_session profile to be detected")
	}
}

func assertProfileKind(t *testing.T, profiles []Profile, name string, want ProfileKind) {
	t.Helper()

	profile, ok := FindProfile(profiles, name)
	if !ok {
		t.Fatalf("profile %q not found", name)
	}
	if profile.Kind != want {
		t.Fatalf("profile %q kind = %q, want %q", name, profile.Kind, want)
	}
}
