package screens

import "github.com/anupgiri/awscan/internal/tui"

func ServiceStep(options []tui.Option, selected string) tui.Step {
	return tui.Step{
		Key:          "service",
		Title:        "Select ECS service",
		Options:      options,
		DefaultValue: selected,
	}
}
