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

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runECSShell(cmd.Context(), env, flags, ecsShellFlags{})
	}

	return cmd
}
