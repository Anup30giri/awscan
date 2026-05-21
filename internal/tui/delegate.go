package tui

import (
	"fmt"
	"io"
	"sort"

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
	title := option.Label
	desc := option.Details
	if index == model.Index() {
		prefix = "› "
		titleStyle = titleStyle.Bold(true).Foreground(lipgloss.Color("86"))
	}
	matches := model.MatchesForItem(index)
	if titleRunes := titleMatches(option.Label, matches); len(titleRunes) > 0 {
		unmatched := titleStyle.Inline(true)
		matched := unmatched.Inherit(model.Styles.DefaultFilterCharacterMatch)
		title = lipgloss.StyleRunes(option.Label, titleRunes, matched, unmatched)
	}
	if descRunes := descMatches(option, matches); len(descRunes) > 0 {
		unmatched := descStyle.Inline(true)
		matched := unmatched.Inherit(model.Styles.DefaultFilterCharacterMatch)
		desc = lipgloss.StyleRunes(option.Details, descRunes, matched, unmatched)
	}

	_, _ = fmt.Fprintf(w, "%s%s\n", prefix, titleStyle.Render(title))
	_, _ = fmt.Fprintf(w, "  %s", descStyle.Render(desc))
}

func titleMatches(label string, matches []int) []int {
	if len(matches) == 0 {
		return nil
	}
	labelRunes := len([]rune(label))
	result := make([]int, 0, len(matches))
	seen := map[int]bool{}
	for _, index := range matches {
		if index < 0 || index >= labelRunes || seen[index] {
			continue
		}
		seen[index] = true
		result = append(result, index)
	}
	sort.Ints(result)
	return result
}

func descMatches(option Option, matches []int) []int {
	if len(matches) == 0 || option.Details == "" {
		return nil
	}
	prefixLen := len([]rune(option.Label + " " + option.Label + " " + option.Value + " " + option.Value + " "))
	descRunes := len([]rune(option.Details))
	result := make([]int, 0, len(matches))
	seen := map[int]bool{}
	for _, index := range matches {
		descIndex := index - prefixLen
		if descIndex < 0 || descIndex >= descRunes || seen[descIndex] {
			continue
		}
		seen[descIndex] = true
		result = append(result, descIndex)
	}
	sort.Ints(result)
	return result
}
