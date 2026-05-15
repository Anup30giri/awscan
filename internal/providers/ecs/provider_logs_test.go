package ecs

import (
	"context"
	"testing"
)

func TestResolveLogTargets(t *testing.T) {
	t.Parallel()

	provider := NewWithClient(fakeAPI{}, "", "", fakeRunner{})
	targets, err := provider.ResolveLogTargets(context.Background(), "cluster", "arn:aws:ecs:ap-south-1:123:task/prod/abc123")
	if err != nil {
		t.Fatalf("ResolveLogTargets() error = %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("len(targets) = %d, want 1", len(targets))
	}
	if targets[0].LogGroup != "/ecs/api" {
		t.Fatalf("LogGroup = %q, want /ecs/api", targets[0].LogGroup)
	}
}
