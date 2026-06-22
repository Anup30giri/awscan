package cmd

import (
	"context"
	"errors"

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
	req := EC2PortForwardRequest{
		Profile:    runtime.Profile,
		Region:     runtime.Region,
		Instance:   flags.instance,
		RemoteHost: firstNonEmpty(flags.remoteHost, env.prefs.Recent.EC2.RemoteHost),
		LocalPort:  flags.localPort,
		RemotePort: flags.remotePort,
	}
	if req.LocalPort == 0 {
		req.LocalPort = env.prefs.Recent.EC2.LocalPort
	}
	if req.RemotePort == 0 {
		req.RemotePort = env.prefs.Recent.EC2.RemotePort
	}

	if !flags.nonInteractive && req.Instance == "" {
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
		req.Instance = output.State.Instance
	}

	if req.Instance == "" || flags.localPort <= 0 || flags.remotePort <= 0 {
		if req.Instance == "" || req.LocalPort <= 0 || req.RemotePort <= 0 {
			return errors.New("instance, local-port, and remote-port are required")
		}
	}
	return executeEC2PortForwardRequest(ctx, env, runtime, provider, req)
}
