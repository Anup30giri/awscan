package cmd

import (
	"testing"

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
