package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	internalaws "github.com/anupgiri/awscan/internal/aws"
	ec2provider "github.com/anupgiri/awscan/internal/providers/ec2"
	"github.com/anupgiri/awscan/internal/tui"
	"github.com/anupgiri/awscan/internal/tui/screens"
	"github.com/spf13/cobra"
)

type ec2ShellFlags struct {
	instance       string
	command        string
	nonInteractive bool
}

func newEC2ShellCommand(env *commandEnv, root *rootFlags) *cobra.Command {
	flags := ec2ShellFlags{}

	cmd := &cobra.Command{
		Use:   "shell",
		Short: "Open an SSM shell into a running EC2 instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEC2Shell(cmd.Context(), env, root, flags)
		},
	}

	cmd.Flags().StringVar(&flags.instance, "instance", "", "EC2 instance ID")
	cmd.Flags().StringVar(&flags.command, "command", "", "Interactive command to run, for example /bin/sh or /bin/bash")
	cmd.Flags().BoolVar(&flags.nonInteractive, "non-interactive", false, "Fail instead of prompting for missing values")

	return cmd
}

func runEC2Shell(ctx context.Context, env *commandEnv, root *rootFlags, flags ec2ShellFlags) error {
	profile := root.profile
	region := root.region

	if !flags.nonInteractive {
		selectedProfile, selectedRegion, err := resolveProfileAndRegionInteractively(ctx, env, profile, region)
		if err != nil {
			return err
		}
		profile = selectedProfile
		region = selectedRegion
	}

	runtime, err := env.resolver.Resolve(ctx, internalaws.ResolveOptions{
		Profile: profile,
		Region:  region,
	})
	if err != nil {
		return err
	}

	provider := ec2provider.New(runtime.Config, env.runner)
	instanceID := flags.instance
	command := flags.command

	if command == "" && instanceID != "" {
		command = env.prefs.DefaultShells[instanceID]
	}

	if flags.nonInteractive {
		if instanceID == "" {
			return errors.New("instance must be provided in --non-interactive mode")
		}
		if command == "" {
			command = "/bin/sh"
		}
	} else {
		instanceID, command, err = resolveEC2SelectionsInteractively(ctx, env, provider, runtime, instanceID, command)
		if err != nil {
			return err
		}
	}

	readiness, err := provider.CheckSessionReadiness(ctx, instanceID)
	if err != nil {
		return err
	}
	if !readiness.ManagedBySSM || strings.ToLower(readiness.PingStatus) != "online" {
		return fmt.Errorf("this EC2 instance is not ready for Session Manager. Ensure SSM Agent is installed, the instance is managed by Systems Manager, and the IAM role allows SSM")
	}

	if err := saveEC2Preferences(env, runtime.Profile, runtime.Region, instanceID, command); err != nil {
		return err
	}

	return provider.StartSession(ctx, ec2provider.StartSessionInput{
		Profile:    runtime.Profile,
		Region:     runtime.Region,
		InstanceID: instanceID,
		Command:    command,
	})
}

func resolveEC2SelectionsInteractively(
	ctx context.Context,
	env *commandEnv,
	provider ec2provider.Provider,
	runtime internalaws.Runtime,
	instanceID, command string,
) (string, string, error) {
	state := tui.WorkflowState{
		Profile:  runtime.Profile,
		Region:   runtime.Region,
		Instance: instanceID,
		Command:  command,
	}

	steps := []tui.Step{}
	if instanceID == "" {
		steps = append(steps, screens.InstanceStep(nil, env.prefs.Recent.EC2.InstanceID))
		steps[len(steps)-1].Load = func(state tui.WorkflowState) ([]tui.Option, error) {
			instances, err := provider.ListInstances(ctx)
			if err != nil {
				return nil, err
			}
			if len(instances) == 0 {
				return nil, fmt.Errorf("no running EC2 instances found in %s", runtime.Region)
			}
			options := make([]tui.Option, 0, len(instances))
			for _, instance := range instances {
				label := firstNonEmpty(instance.Name, instance.InstanceID)
				details := fmt.Sprintf("%s | private=%s | az=%s | ssm=%t",
					instance.Platform, firstNonEmpty(instance.PrivateIP, "-"), firstNonEmpty(instance.AvailabilityAZ, "-"), instance.ManagedBySSM)
				options = append(options, tui.Option{
					Label:   label,
					Details: details,
					Value:   instance.InstanceID,
				})
			}
			return options, nil
		}
	}

	if strings.TrimSpace(command) == "" {
		remembered := ""
		if instanceID != "" {
			remembered = env.prefs.DefaultShells[instanceID]
		}
		steps = append(steps, screens.CommandStep(buildCommandOptions(), remembered))
	}

	if len(steps) == 0 {
		if command == "" {
			command = "/bin/sh"
		}
		return instanceID, command, nil
	}

	output, err := tui.RunWorkflow(ctx, tui.WorkflowInput{
		Title: "awscan ec2 shell",
		Steps: steps,
		State: state,
	})
	if err != nil {
		return "", "", err
	}

	finalState := output.State
	selectedInstance := firstNonEmpty(finalState.Instance, instanceID)
	selectedCommand := firstNonEmpty(finalState.Command, command, env.prefs.DefaultShells[selectedInstance], "/bin/sh")
	return selectedInstance, selectedCommand, nil
}

func saveEC2Preferences(env *commandEnv, profile, region, instanceID, command string) error {
	env.prefs.DefaultProfile = profile
	env.prefs.DefaultRegion = region
	env.prefs.Recent.EC2.InstanceID = instanceID
	if instanceID != "" && command != "" {
		if env.prefs.DefaultShells == nil {
			env.prefs.DefaultShells = map[string]string{}
		}
		env.prefs.DefaultShells[instanceID] = command
	}
	return env.app.Config.Save(env.prefs)
}
