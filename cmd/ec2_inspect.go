package cmd

import (
	"context"
	"fmt"
	"strings"

	ec2provider "github.com/anupgiri/awscan/internal/providers/ec2"
	"github.com/anupgiri/awscan/internal/tui"
	"github.com/spf13/cobra"
)

type ec2InspectFlags struct {
	instance       string
	nonInteractive bool
}

func newEC2InspectCommand(env *commandEnv, root *rootFlags) *cobra.Command {
	flags := ec2InspectFlags{}
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect EC2 instance and SSM readiness details",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEC2Inspect(cmd.Context(), env, root, flags)
		},
	}
	cmd.Flags().StringVar(&flags.instance, "instance", "", "EC2 instance ID or Name tag")
	cmd.Flags().BoolVar(&flags.nonInteractive, "non-interactive", false, "Fail instead of prompting for missing values")
	return cmd
}

func runEC2Inspect(ctx context.Context, env *commandEnv, root *rootFlags, flags ec2InspectFlags) error {
	runtime, err := resolveShellRuntime(ctx, env, root, flags.nonInteractive)
	if err != nil {
		return err
	}
	provider := ec2provider.New(runtime.Config, env.runner)
	adapter := runtimeAdapter{profile: runtime.Profile, region: runtime.Region, account: accountID(runtime)}
	instanceID, err := resolveEC2InstanceSelection(ctx, env, provider, adapter, flags.instance, flags.nonInteractive, "awscan ec2 inspect")
	if err != nil {
		return err
	}
	detail, err := provider.DescribeInstance(ctx, instanceID)
	if err != nil {
		return err
	}
	readiness, err := provider.CheckSessionReadiness(ctx, instanceID)
	if err != nil {
		return err
	}
	fmt.Print(renderEC2Inspect(detail, readiness))
	return nil
}

func renderEC2Inspect(detail *ec2provider.InstanceDetail, readiness *ec2provider.SessionReadiness) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Instance: %s\nName: %s\nState: %s\nPlatform: %s\nType: %s\nPrivate IP: %s\nPublic IP: %s\nAZ: %s\nSubnet: %s\nVPC: %s\nArchitecture: %s\nIAM Profile: %s\nSSM Managed: %t\nSSM Ping: %s\n",
		detail.InstanceID, firstNonEmpty(detail.Name, "-"), detail.State, detail.Platform, detail.InstanceType, firstNonEmpty(detail.PrivateIP, "-"),
		firstNonEmpty(detail.PublicIP, "-"), firstNonEmpty(detail.AvailabilityAZ, "-"), firstNonEmpty(detail.SubnetID, "-"),
		firstNonEmpty(detail.VpcID, "-"), firstNonEmpty(detail.Architecture, "-"), firstNonEmpty(detail.IAMProfileARN, "-"),
		readiness.ManagedBySSM, firstNonEmpty(readiness.PingStatus, "-"))
	if len(detail.SecurityGroups) > 0 {
		b.WriteString("Security Groups:\n")
		for _, group := range detail.SecurityGroups {
			fmt.Fprintf(&b, "- %s\n", group)
		}
	}
	if len(readiness.Warnings) > 0 {
		b.WriteString("Warnings:\n")
		for _, warning := range readiness.Warnings {
			fmt.Fprintf(&b, "- %s\n", warning)
		}
	}
	return b.String()
}

func resolveEC2InstanceSelection(ctx context.Context, env *commandEnv, provider ec2provider.Provider, runtime runtimeAdapter, instance string, nonInteractive bool, title string) (string, error) {
	instanceID := instance
	var err error
	if instanceID != "" {
		instanceID, err = provider.ResolveInstanceID(ctx, instanceID)
		if err != nil {
			return "", err
		}
	}
	if nonInteractive {
		if instanceID == "" {
			return "", fmt.Errorf("instance must be provided in --non-interactive mode")
		}
		return instanceID, nil
	}
	if instanceID != "" {
		return instanceID, nil
	}
	output, err := tui.RunWorkflow(ctx, tui.WorkflowInput{
		Title: title,
		State: tui.WorkflowState{
			Profile: runtime.ProfileName(),
			Region:  runtime.RegionName(),
			Account: runtime.AccountID(),
			Target:  "ec2",
		},
		Steps: []tui.Step{buildEC2InstanceStep(ctx, env, provider, runtime, "")},
	})
	if err != nil {
		return "", err
	}
	return output.State.Instance, nil
}
