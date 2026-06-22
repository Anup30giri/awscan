package cmd

import (
	"testing"

	appconfig "github.com/anupgiri/awscan/internal/config"
	"github.com/anupgiri/awscan/pkg/plugin"
)

func TestServiceTargetOptions(t *testing.T) {
	t.Parallel()

	options := serviceTargetOptionsFromRegistry([]plugin.ServiceRegistration{
		{ID: "ecs", Name: "ECS", Description: "ecs"},
		{ID: "ec2", Name: "EC2", Description: "ec2"},
	})
	if len(options) != 2 {
		t.Fatalf("len(options) = %d, want 2", len(options))
	}
	if options[0].Value != "ecs" || options[1].Value != "ec2" {
		t.Fatalf("unexpected options: %+v", options)
	}
}

func TestServiceCommandByID(t *testing.T) {
	t.Parallel()

	services := []plugin.ServiceRegistration{
		{ID: "ecs", Name: "ECS"},
	}
	service, err := serviceCommandByID(services, "ecs")
	if err != nil {
		t.Fatalf("serviceCommandByID() error = %v", err)
	}
	if service.Name != "ECS" {
		t.Fatalf("service.Name = %q, want ECS", service.Name)
	}
}

func TestDefaultTargetOptionsIncludesSavedWhenConfigured(t *testing.T) {
	t.Parallel()

	prefs := &appconfig.Preferences{
		Saved: map[string]appconfig.SavedWorkflow{
			"prod-api-shell": {Kind: appconfig.SavedWorkflowKindECSShell},
		},
	}
	options := defaultTargetOptions(prefs, []plugin.ServiceRegistration{
		{ID: "ecs", Name: "ECS", Description: "ecs"},
	})
	if len(options) == 0 || options[0].Value != "saved" {
		t.Fatalf("expected first target option to be saved, got %+v", options)
	}
}
