package screens

import "github.com/anupgiri/awscan/internal/tui"

func CommandStep(options []tui.Option, selected string) tui.Step {
	return tui.Step{
		Key:          "command",
		Title:        "Select shell command or type custom command",
		Placeholder:  "search command, for example bash or sh",
		AllowCustom:  true,
		Options:      options,
		DefaultValue: selected,
	}
}
