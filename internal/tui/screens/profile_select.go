package screens

import "github.com/anupgiri/awscan/internal/tui"

func ProfileStep(options []tui.Option, selected string) tui.Step {
	return tui.Step{
		Key:          "profile",
		Title:        "Select AWS profile",
		Placeholder:  "search profile by name, type, or region",
		Options:      options,
		DefaultValue: selected,
	}
}
