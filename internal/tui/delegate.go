package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type optionDelegate struct{}

func (d optionDelegate) Height() int                             { return 2 }
func (d optionDelegate) Spacing() int                            { return 1 }
func (d optionDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d optionDelegate) Render(w io.Writer, model list.Model, index int, item list.Item) {
	option, ok := item.(Option)
	if !ok {
		return
	}

	prefix := "  "
	titleStyle := lipgloss.NewStyle()
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	if index == model.Index() {
		prefix = "› "
		titleStyle = titleStyle.Bold(true).Foreground(lipgloss.Color("86"))
	}

	_, _ = fmt.Fprintf(w, "%s%s\n", prefix, titleStyle.Render(option.Label))
	_, _ = fmt.Fprintf(w, "  %s", descStyle.Render(option.Details))
}
