package screens

import "github.com/anupgiri/awscan/internal/tui"

func TargetStep(options []tui.Option, selected string) tui.Step {
	return tui.Step{
		Key:          "target",
		Title:        "Select service",
		Options:      options,
		DefaultValue: selected,
	}
}
