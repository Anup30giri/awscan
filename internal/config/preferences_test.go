package config

import (
	"path/filepath"
	"testing"
)

func TestPreferencesSavedRoundTrip(t *testing.T) {
	t.Parallel()

	manager := &Manager{path: filepath.Join(t.TempDir(), "config.yaml")}
	input := &Preferences{
		DefaultProfile: "default",
		DefaultRegion:  "ap-south-1",
		Saved: map[string]SavedWorkflow{
			"prod-api-shell": {
				Kind:      SavedWorkflowKindECSShell,
				Profile:   "default",
				Region:    "ap-south-1",
				Cluster:   "prod",
				Service:   "api",
				Container: "app",
				Command:   "/bin/sh",
			},
		},
	}
	if err := manager.Save(input); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	output, err := manager.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(output.Saved) != 1 {
		t.Fatalf("len(output.Saved) = %d, want 1", len(output.Saved))
	}
	if output.Saved["prod-api-shell"].Kind != SavedWorkflowKindECSShell {
		t.Fatalf("kind = %q, want %q", output.Saved["prod-api-shell"].Kind, SavedWorkflowKindECSShell)
	}
}

func TestValidateSavedWorkflowName(t *testing.T) {
	t.Parallel()

	if err := ValidateSavedWorkflowName("prod-api-shell"); err != nil {
		t.Fatalf("ValidateSavedWorkflowName() error = %v", err)
	}
	if err := ValidateSavedWorkflowName("Prod API"); err == nil {
		t.Fatal("expected invalid saved workflow name")
	}
}

func TestSavedWorkflowValidate(t *testing.T) {
	t.Parallel()

	workflow := SavedWorkflow{
		Kind:      SavedWorkflowKindECSLogs,
		Profile:   "default",
		Region:    "ap-south-1",
		Cluster:   "prod",
		Service:   "api",
		Container: "app",
	}
	if err := workflow.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	workflow.Container = ""
	if err := workflow.Validate(); err == nil {
		t.Fatal("expected validation error for missing container")
	}
}
