package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	ec2provider "github.com/anupgiri/awscan/internal/providers/ec2"
	"github.com/anupgiri/awscan/internal/tui"
	"github.com/spf13/cobra"
)

type ec2PortForwardFlags struct {
	instance       string
	remoteHost     string
	localPort      int
	remotePort     int
	nonInteractive bool
}

func newEC2PortForwardCommand(env *commandEnv, root *rootFlags) *cobra.Command {
	flags := ec2PortForwardFlags{}
	cmd := &cobra.Command{
		Use:   "port-forward",
		Short: "Start SSM port forwarding to EC2 instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEC2PortForward(cmd.Context(), env, root, flags)
		},
	}
	cmd.Flags().StringVar(&flags.instance, "instance", "", "EC2 instance ID or Name tag")
	cmd.Flags().StringVar(&flags.remoteHost, "remote-host", "", "Optional remote host behind instance")
	cmd.Flags().IntVar(&flags.localPort, "local-port", 0, "Local port to bind")
	cmd.Flags().IntVar(&flags.remotePort, "remote-port", 0, "Remote port to forward to")
	cmd.Flags().BoolVar(&flags.nonInteractive, "non-interactive", false, "Fail instead of prompting for missing values")
	return cmd
}

func runEC2PortForward(ctx context.Context, env *commandEnv, root *rootFlags, flags ec2PortForwardFlags) error {
	runtime, err := resolveShellRuntime(ctx, env, root, flags.nonInteractive)
	if err != nil {
		return err
	}

	provider := ec2provider.New(runtime.Config, env.runner)
	instanceID := flags.instance
	if instanceID != "" {
		instanceID, err = provider.ResolveInstanceID(ctx, instanceID)
		if err != nil {
			return err
		}
	}

	if !flags.nonInteractive && instanceID == "" {
		adapter := runtimeAdapter{profile: runtime.Profile, region: runtime.Region, account: accountID(runtime)}
		output, err := tui.RunWorkflow(ctx, tui.WorkflowInput{
			Title: "awscan ec2 port-forward",
			State: tui.WorkflowState{
				Profile: runtime.Profile,
				Region:  runtime.Region,
				Account: accountID(runtime),
				Target:  "ec2",
			},
			Steps: []tui.Step{buildEC2InstanceStep(ctx, env, provider, adapter, "")},
		})
		if err != nil {
			return err
		}
		instanceID = output.State.Instance
	}

	if instanceID == "" || flags.localPort <= 0 || flags.remotePort <= 0 {
		return errors.New("instance, local-port, and remote-port are required")
	}

	readiness, err := provider.CheckSessionReadiness(ctx, instanceID)
	if err != nil {
		return err
	}
	if !readiness.ManagedBySSM || strings.ToLower(readiness.PingStatus) != "online" {
		return fmt.Errorf("this EC2 instance is not ready for Session Manager. Ensure SSM Agent is installed, instance is managed by Systems Manager, and IAM role allows SSM")
	}

	saveGlobalPreferences(env, runtime.Profile, runtime.Region)
	env.prefs.Recent.EC2.InstanceID = instanceID
	if err := env.app.Config.Save(env.prefs); err != nil {
		return err
	}

	return provider.StartPortForward(ctx, ec2provider.StartPortForwardInput{
		Profile:    runtime.Profile,
		Region:     runtime.Region,
		InstanceID: instanceID,
		LocalPort:  flags.localPort,
		RemotePort: flags.remotePort,
		RemoteHost: flags.remoteHost,
	})
}
