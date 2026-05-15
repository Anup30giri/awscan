package ec2

import (
	"context"
	"testing"
)

func TestResolveInstanceIDFromName(t *testing.T) {
	t.Parallel()

	provider := NewWithClients(fakeEC2API{}, fakeSSMAPI{}, &fakeRunner{})
	got, err := provider.ResolveInstanceID(context.Background(), "api-node")
	if err != nil {
		t.Fatalf("ResolveInstanceID() error = %v", err)
	}
	if got != "i-1234567890" {
		t.Fatalf("ResolveInstanceID() = %q, want i-1234567890", got)
	}
}
