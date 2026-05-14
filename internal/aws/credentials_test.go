package aws

import "testing"

func TestParseProcessCredentialDocument(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"Version":1,"AccessKeyId":"AKIA","SecretAccessKey":"SECRET","SessionToken":"TOKEN","Expiration":"2026-05-14T10:05:11Z"}`)
	doc, err := ParseProcessCredentialDocument(raw)
	if err != nil {
		t.Fatalf("ParseProcessCredentialDocument() error = %v", err)
	}

	if doc.AccessKeyID != "AKIA" {
		t.Fatalf("AccessKeyID = %q, want AKIA", doc.AccessKeyID)
	}
}

func TestParseProcessCredentialDocumentRejectsInvalidVersion(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"Version":2,"AccessKeyId":"AKIA","SecretAccessKey":"SECRET"}`)
	if _, err := ParseProcessCredentialDocument(raw); err == nil {
		t.Fatal("expected invalid version error")
	}
}
