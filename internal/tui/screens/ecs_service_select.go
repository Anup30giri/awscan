package screens

import "github.com/anupgiri/awscan/internal/tui"

func ServiceStep(options []tui.Option, selected string) tui.Step {
	return tui.Step{
		Key:          "service",
		Title:        "Select ECS service",
		Placeholder:  "search service by name, counts, or exec state",
		Options:      options,
		DefaultValue: selected,
	}
}
