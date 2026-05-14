package screens

import "github.com/anupgiri/awscan/internal/tui"

func ProfileStep(options []tui.Option, selected string) tui.Step {
	return tui.Step{
		Key:          "profile",
		Title:        "Select AWS profile",
		Options:      options,
		DefaultValue: selected,
	}
}
