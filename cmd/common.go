package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	internalaws "github.com/anupgiri/awscan/internal/aws"
	appconfig "github.com/anupgiri/awscan/internal/config"
	"github.com/anupgiri/awscan/internal/tui"
	"github.com/anupgiri/awscan/internal/tui/screens"
	"github.com/anupgiri/awscan/pkg/plugin"
)

func resolveShellRuntime(ctx context.Context, env *commandEnv, root *rootFlags, nonInteractive bool) (internalaws.Runtime, error) {
	runtime, profile, region, err := resolveRuntime(ctx, env, root.profile, root.region, !nonInteractive)
	if err != nil {
		return internalaws.Runtime{}, err
	}
	root.profile = profile
	root.region = region
	return runtime, nil
}

func resolveProfileAndRegionInteractively(ctx context.Context, env *commandEnv, profile, region string) (string, string, error) {
	if profile != "" && region != "" {
		return profile, region, nil
	}

	profiles, err := internalaws.LoadProfiles(internalaws.DefaultSharedConfigPaths())
	if err != nil {
		return "", "", err
	}

	state := tui.WorkflowState{
		Profile: profile,
		Region:  region,
	}

	steps := []tui.Step{}
	if profile == "" {
		options := buildProfileOptions(profiles)
		if len(options) == 0 {
			return "", "", errors.New("no AWS profiles or environment credentials were found. Run `aws login`, `aws sso login`, or set environment credentials first")
		}
		steps = append(steps, screens.ProfileStep(options, env.prefs.DefaultProfile))
	}
	if region == "" {
		steps = append(steps, screens.RegionStep(buildRegionOptions(), env.prefs.DefaultRegion))
	}
	if len(steps) == 0 {
		return profile, region, nil
	}

	output, err := tui.RunWorkflow(ctx, tui.WorkflowInput{
		Title: "awscan",
		Steps: steps,
		State: state,
	})
	if err != nil {
		return "", "", err
	}

	return firstNonEmpty(output.State.Profile, profile), firstNonEmpty(output.State.Region, region), nil
}

func buildProfileOptions(profiles []internalaws.Profile) []tui.Option {
	options := make([]tui.Option, 0, len(profiles)+1)
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != "" {
		options = append(options, tui.Option{
			Label:   "environment",
			Details: "Use AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY from current shell",
			Value:   "",
		})
	}
	for _, profile := range profiles {
		details := fmt.Sprintf("type=%s region=%s", profile.Kind, firstNonEmpty(profile.Region, "-"))
		options = append(options, tui.Option{
			Label:   profile.Name,
			Details: details,
			Value:   profile.Name,
			Meta: map[string]string{
				"type":   string(profile.Kind),
				"region": profile.Region,
			},
		})
	}
	return options
}

func buildRegionOptions() []tui.Option {
	regions := internalaws.KnownRegions()
	options := make([]tui.Option, 0, len(regions))
	for _, region := range regions {
		options = append(options, tui.Option{
			Label:   region,
			Details: "AWS region",
			Value:   region,
			Meta: map[string]string{
				"region": region,
			},
		})
	}
	return options
}

func buildCommandOptions() []tui.Option {
	return []tui.Option{
		{Label: "/bin/sh", Details: "Portable POSIX shell", Value: "/bin/sh"},
		{Label: "/bin/bash", Details: "Bash shell if present", Value: "/bin/bash"},
	}
}

func selectDefaultTarget(ctx context.Context, prefs *appconfig.Preferences, runtime internalaws.Runtime, services []plugin.ServiceRegistration) (string, error) {
	targetOptions := defaultTargetOptions(prefs, services)
	output, err := tui.RunWorkflow(ctx, tui.WorkflowInput{
		Title: "awscan",
		State: tui.WorkflowState{
			Profile: runtime.Profile,
			Region:  runtime.Region,
			Account: accountID(runtime),
		},
		Steps: []tui.Step{
			screens.TargetStep(targetOptions, ""),
		},
	})
	if err != nil {
		return "", err
	}
	if output.State.Target != "saved" {
		return output.State.Target, nil
	}

	savedOptions := savedWorkflowOptions(prefs)
	savedOutput, err := tui.RunWorkflow(ctx, tui.WorkflowInput{
		Title: "awscan saved workflows",
		State: tui.WorkflowState{
			Profile: runtime.Profile,
			Region:  runtime.Region,
			Account: accountID(runtime),
			Target:  "saved",
		},
		Steps: []tui.Step{
			screens.TargetStep(savedOptions, ""),
		},
	})
	if err != nil {
		return "", err
	}
	return savedOutput.State.Target, nil
}

func defaultTargetOptions(prefs *appconfig.Preferences, services []plugin.ServiceRegistration) []tui.Option {
	targetOptions := serviceTargetOptionsFromRegistry(services)
	if len(prefs.Saved) == 0 {
		return targetOptions
	}
	return append([]tui.Option{{
		Label:   "Saved workflows",
		Details: "Run a named saved workflow without navigating resources again",
		Value:   "saved",
		Meta: map[string]string{
			"service": "saved",
		},
	}}, targetOptions...)
}

func saveGlobalPreferences(env *commandEnv, profile, region string) {
	env.prefs.DefaultProfile = profile
	env.prefs.DefaultRegion = region
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func accountID(runtime internalaws.Runtime) string {
	if runtime.Identity == nil {
		return ""
	}
	return runtime.Identity.Account
}
