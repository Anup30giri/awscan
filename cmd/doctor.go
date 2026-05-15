package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/anupgiri/awscan/internal/diagnostics"
)

func newDoctorCommand(env *commandEnv, root *rootFlags) *cobra.Command {
	var flags diagnostics.Options

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run local AWS and ECS diagnostics",
		RunE: func(cmd *cobra.Command, args []string) error {
			flags.Profile = firstNonEmpty(root.profile, flags.Profile)
			flags.Region = firstNonEmpty(root.region, flags.Region)

			doctor := diagnostics.NewDoctor(env.resolver, env.runner)
			report, err := doctor.Run(cmd.Context(), flags)
			if err != nil {
				return err
			}

			fmt.Print(report.String())
			return nil
		},
	}

	cmd.Flags().StringVar(&flags.Cluster, "cluster", "", "ECS cluster ARN/name for exec readiness checks")
	cmd.Flags().StringVar(&flags.Service, "service", "", "ECS service ARN/name for exec readiness checks")
	cmd.Flags().StringVar(&flags.Task, "task", "", "ECS task ARN/id for exec readiness checks")
	cmd.Flags().StringVar(&flags.Instance, "instance", "", "EC2 instance ID for Session Manager readiness checks")

	return cmd
}
