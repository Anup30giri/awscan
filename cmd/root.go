package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/anupgiri/awscan/internal/app"
	internalaws "github.com/anupgiri/awscan/internal/aws"
	appconfig "github.com/anupgiri/awscan/internal/config"
	internalexec "github.com/anupgiri/awscan/internal/exec"
	"github.com/anupgiri/awscan/internal/tui"
)

type commandEnv struct {
	app      *app.App
	prefs    *appconfig.Preferences
	resolver internalaws.Resolver
	runner   internalexec.Runner
}

type rootFlags struct {
	profile string
	region  string
}

func Execute(ctx context.Context) error {
	application, err := app.New()
	if err != nil {
		return err
	}

	prefs, err := application.Config.Load()
	if err != nil {
		return err
	}

	env := &commandEnv{
		app:      application,
		prefs:    prefs,
		resolver: internalaws.NewConfigResolver(prefs),
		runner:   internalexec.NewRunner(),
	}

	root := newRootCommand(env)
	root.SetContext(ctx)
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}

func newRootCommand(env *commandEnv) *cobra.Command {
	flags := &rootFlags{}

	cmd := &cobra.Command{
		Use:           "awscan",
		Short:         "AWS terminal navigator focused on ECS Exec workflows",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVar(&flags.profile, "profile", "", "AWS profile to use")
	cmd.PersistentFlags().StringVar(&flags.region, "region", "", "AWS region to use")

	cmd.AddCommand(newDoctorCommand(env, flags))
	cmd.AddCommand(newECSCommand(env, flags))
	cmd.AddCommand(newEC2Command(env, flags))

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		profile := flags.profile
		region := flags.region

		selectedProfile, selectedRegion, err := resolveProfileAndRegionInteractively(cmd.Context(), env, profile, region)
		if err != nil {
			return err
		}
		flags.profile = selectedProfile
		flags.region = selectedRegion

		target, err := selectDefaultShellTarget(cmd.Context(), selectedProfile, selectedRegion)
		if err != nil {
			return err
		}

		switch target {
		case "ecs":
			return runECSShell(cmd.Context(), env, flags, ecsShellFlags{})
		case "ec2":
			return runEC2Shell(cmd.Context(), env, flags, ec2ShellFlags{})
		default:
			return fmt.Errorf("unsupported shell target %q", target)
		}
	}

	return cmd
}

func selectDefaultShellTarget(ctx context.Context, profile, region string) (string, error) {
	output, err := tui.RunWorkflow(ctx, tui.WorkflowInput{
		Title: "awscan",
		State: tui.WorkflowState{
			Profile: profile,
			Region:  region,
		},
		Steps: []tui.Step{{
			Key:   "command",
			Title: "Select service",
			Options: []tui.Option{
				{
					Label:   "ECS",
					Details: "Open a shell into a running ECS container using ECS Exec",
					Value:   "ecs",
				},
				{
					Label:   "EC2",
					Details: "Open a shell into a running EC2 instance using Session Manager",
					Value:   "ec2",
				},
			},
		}},
	})
	if err != nil {
		return "", err
	}

	return output.State.Command, nil
}
