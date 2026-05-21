package screens

import "github.com/anupgiri/awscan/internal/tui"

func TargetStep(options []tui.Option, selected string) tui.Step {
	return tui.Step{
		Key:          "target",
		Title:        "Select service",
		Placeholder:  "search target service, for example ecs or ec2",
		Options:      options,
		DefaultValue: selected,
	}
}
