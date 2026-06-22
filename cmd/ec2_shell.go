package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	runtime, err := resolveShellRuntime(ctx, env, root, flags.nonInteractive)
	if err != nil {
		return err
	}

	provider := ec2provider.New(runtime.Config, env.runner)
	req := EC2ShellRequest{
		Profile:  runtime.Profile,
		Region:   runtime.Region,
		Instance: flags.instance,
		Command:  flags.command,
	}
	command := req.Command

	if command == "" && req.Instance != "" {
		command = env.prefs.DefaultShells[req.Instance]
	}
	req.Command = command

	if flags.nonInteractive {
		if req.Instance == "" {
			return errors.New("instance must be provided in --non-interactive mode")
		}
	} else {
		req.Instance, req.Command, err = resolveEC2SelectionsInteractively(ctx, env, provider, runtimeAdapter{profile: runtime.Profile, region: runtime.Region, account: accountID(runtime)}, req.Instance, req.Command)
		if err != nil {
			return err
		}
	}
	return executeEC2ShellRequest(ctx, env, runtime, provider, req)
}

func resolveEC2SelectionsInteractively(
	ctx context.Context,
	env *commandEnv,
	provider ec2provider.Provider,
	runtime runtimeAdapter,
	instanceID, command string,
) (string, string, error) {
	state := tui.WorkflowState{
		Profile:  runtime.ProfileName(),
		Region:   runtime.RegionName(),
		Account:  runtime.AccountID(),
		Target:   "ec2",
		Instance: instanceID,
		Command:  command,
	}

	steps := []tui.Step{}
	if instanceID == "" {
		steps = append(steps, buildEC2InstanceStep(ctx, env, provider, runtime, instanceID))
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
	saveGlobalPreferences(env, profile, region)
	env.prefs.Recent.EC2.InstanceID = instanceID
	if instanceID != "" && command != "" {
		if env.prefs.DefaultShells == nil {
			env.prefs.DefaultShells = map[string]string{}
		}
		env.prefs.DefaultShells[instanceID] = command
	}
	return env.app.Config.Save(env.prefs)
}

func buildEC2InstanceStep(ctx context.Context, env *commandEnv, provider ec2provider.Provider, runtime runtimeAdapter, instanceID string) tui.Step {
	step := screens.InstanceStep(nil, env.prefs.Recent.EC2.InstanceID)
	step.Load = func(state tui.WorkflowState) ([]tui.Option, error) {
		instances, err := provider.ListInstances(ctx)
		if err != nil {
			return nil, err
		}
		if len(instances) == 0 {
			return nil, fmt.Errorf("no running EC2 instances found in %s", runtime.RegionName())
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
				Meta: map[string]string{
					"id":       instance.InstanceID,
					"name":     instance.Name,
					"private":  instance.PrivateIP,
					"public":   instance.PublicIP,
					"platform": instance.Platform,
					"state":    instance.State,
					"az":       instance.AvailabilityAZ,
					"ssm":      fmt.Sprintf("%t", instance.ManagedBySSM),
				},
			})
		}
		return options, nil
	}
	return step
}
