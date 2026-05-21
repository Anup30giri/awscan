package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	ec2provider "github.com/anupgiri/awscan/internal/providers/ec2"
	"github.com/spf13/cobra"
)

type ec2DocumentsFlags struct {
	instance       string
	document       string
	commands       []string
	yes            bool
	nonInteractive bool
}

func newEC2DocumentsCommand(env *commandEnv, root *rootFlags) *cobra.Command {
	flags := ec2DocumentsFlags{}
	cmd := &cobra.Command{
		Use:   "documents",
		Short: "List or execute a bounded allowlist of SSM documents",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEC2Documents(cmd.Context(), env, root, flags)
		},
	}
	cmd.Flags().StringVar(&flags.instance, "instance", "", "EC2 instance ID or Name tag")
	cmd.Flags().StringVar(&flags.document, "document", "", "Allowlisted SSM document name")
	cmd.Flags().StringArrayVar(&flags.commands, "command", nil, "Command lines for AWS-RunShellScript or AWS-RunPowerShellScript")
	cmd.Flags().BoolVar(&flags.yes, "yes", false, "Skip confirmation prompt for document execution")
	cmd.Flags().BoolVar(&flags.nonInteractive, "non-interactive", false, "Fail instead of prompting for missing values")
	return cmd
}

func runEC2Documents(ctx context.Context, env *commandEnv, root *rootFlags, flags ec2DocumentsFlags) error {
	runtime, err := resolveShellRuntime(ctx, env, root, flags.nonInteractive)
	if err != nil {
		return err
	}
	provider := ec2provider.New(runtime.Config, env.runner)

	if strings.TrimSpace(flags.document) == "" {
		docs, err := provider.ListSSMDocuments(ctx)
		if err != nil {
			return err
		}
		for _, doc := range docs {
			fmt.Printf("%s type=%s owner=%s platforms=%s\n", doc.Name, doc.DocumentType, doc.Owner, strings.Join(doc.PlatformTypes, ","))
		}
		return nil
	}

	instanceID, err := resolveEC2InstanceSelection(ctx, env, provider, runtimeAdapter{profile: runtime.Profile, region: runtime.Region, account: accountID(runtime)}, flags.instance, flags.nonInteractive, "awscan ec2 documents")
	if err != nil {
		return err
	}
	if !flags.yes {
		if flags.nonInteractive {
			return fmt.Errorf("--yes is required in --non-interactive mode when executing a document")
		}
		confirmed, err := confirmAction(os.Stdin, os.Stdout, fmt.Sprintf("Execute document %s on %s?", flags.document, instanceID))
		if err != nil {
			return err
		}
		if !confirmed {
			return fmt.Errorf("document execution cancelled")
		}
	}
	commandID, err := provider.SendDocumentCommand(ctx, ec2provider.SendDocumentCommandInput{
		Profile:    runtime.Profile,
		Region:     runtime.Region,
		InstanceID: instanceID,
		Document:   flags.document,
		Commands:   flags.commands,
	})
	if err != nil {
		return err
	}
	fmt.Printf("SSM command started: %s\n", commandID)
	return nil
}
