package cmd

import (
	"context"
	"fmt"

	"github.com/anupgiri/awscan/internal/tui"
	"github.com/anupgiri/awscan/pkg/plugin"
	"github.com/spf13/cobra"
)

func registeredServices(env *commandEnv, root *rootFlags) []plugin.ServiceRegistration {
	return []plugin.ServiceRegistration{
		{
			ID:          "ecs",
			Name:        "ECS",
			Description: "Shell, logs, inspect, events, and restart for ECS services and tasks",
			BuildRoot: func() *cobra.Command {
				return newECSCommand(env, root)
			},
			DefaultRun: func(ctx context.Context) error {
				return runECSShell(ctx, env, root, ecsShellFlags{})
			},
		},
		{
			ID:          "ec2",
			Name:        "EC2",
			Description: "Shell, inspect, documents, and port forward for EC2 instances via SSM",
			BuildRoot: func() *cobra.Command {
				return newEC2Command(env, root)
			},
			DefaultRun: func(ctx context.Context) error {
				return runEC2Shell(ctx, env, root, ec2ShellFlags{})
			},
		},
	}
}

func serviceTargetOptionsFromRegistry(registrations []plugin.ServiceRegistration) []tui.Option {
	options := make([]tui.Option, 0, len(registrations))
	for _, registration := range registrations {
		label, details, value := registration.TargetOption()
		options = append(options, tui.Option{
			Label:   label,
			Details: details,
			Value:   value,
			Meta: map[string]string{
				"service": registration.ID,
			},
		})
	}
	return options
}

func serviceCommandByID(registrations []plugin.ServiceRegistration, id string) (*plugin.ServiceRegistration, error) {
	for i := range registrations {
		if registrations[i].ID == id {
			return &registrations[i], nil
		}
	}
	return nil, fmt.Errorf("unsupported shell target %q", id)
}
