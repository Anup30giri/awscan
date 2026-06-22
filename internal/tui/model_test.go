package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWorkflowModelAdvancesAndBacktracks(t *testing.T) {
	t.Parallel()

	m, err := newModel(WorkflowInput{
		Title: "test",
		Steps: []Step{
			{Key: "profile", Title: "Profile", Options: []Option{{Label: "default", Value: "default"}}},
			{Key: "region", Title: "Region", Options: []Option{{Label: "ap-south-1", Value: "ap-south-1"}}},
		},
	})
	if err != nil {
		t.Fatalf("newModel() error = %v", err)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	current := updated.(model)
	if current.state.Profile != "default" {
		t.Fatalf("Profile = %q, want default", current.state.Profile)
	}

	updated, _ = current.Update(tea.KeyMsg{Type: tea.KeyEsc})
	current = updated.(model)
	if current.index != 0 {
		t.Fatalf("index = %d, want 0", current.index)
	}
}

func TestOptionFilterValueIncludesMeta(t *testing.T) {
	t.Parallel()

	option := Option{
		Label:   "api-node",
		Details: "Linux | private=10.0.0.10",
		Value:   "i-1234567890",
		Meta: map[string]string{
			"az":    "ap-south-1a",
			"state": "running",
		},
	}

	got := option.FilterValue()
	for _, want := range []string{"api-node", "10.0.0.10", "i-1234567890", "ap-south-1a", "running"} {
		if !strings.Contains(got, want) {
			t.Fatalf("FilterValue() missing %q in %q", want, got)
		}
	}
}

func TestFilterPlaceholderUsesStepPlaceholder(t *testing.T) {
	t.Parallel()

	step := Step{Placeholder: "search profile by name"}
	if got := filterPlaceholder(step); got != "search profile by name" {
		t.Fatalf("filterPlaceholder() = %q", got)
	}
}

func TestWeightedFilterPrefersLabel(t *testing.T) {
	t.Parallel()

	ranks := weightedFilter("api", []string{
		"node node i-1 i-1 linux api meta",
		"api api i-2 i-2 linux other meta",
	})
	if len(ranks) < 2 {
		t.Fatalf("len(ranks) = %d, want >= 2", len(ranks))
	}
	if ranks[0].Index != 1 {
		t.Fatalf("first rank index = %d, want 1", ranks[0].Index)
	}
}

func TestWeightedFilterMatchesHyphenatedRegion(t *testing.T) {
	t.Parallel()

	targets := []string{
		"ap-south-1 ap-south-1 AWS region region ap-south-1",
		"us-east-1 us-east-1 AWS region region us-east-1",
	}

	ranks := weightedFilter("us-east-1", targets)
	if len(ranks) == 0 {
		t.Fatal("weightedFilter() returned no matches")
	}
	if ranks[0].Index != 1 {
		t.Fatalf("first rank index = %d, want 1", ranks[0].Index)
	}
}

func TestWeightedFilterMatchesNormalizedRegion(t *testing.T) {
	t.Parallel()

	targets := []string{
		"us-east-1 us-east-1 AWS region region us-east-1",
	}

	ranks := weightedFilter("useast1", targets)
	if len(ranks) == 0 {
		t.Fatal("weightedFilter() returned no matches")
	}
	if ranks[0].Index != 0 {
		t.Fatalf("first rank index = %d, want 0", ranks[0].Index)
	}
}

func TestMatchSummaryIncludesCustomHint(t *testing.T) {
	t.Parallel()

	m, err := newModel(WorkflowInput{
		Title: "test",
		Steps: []Step{
			{Key: "command", Title: "Command", AllowCustom: true, Options: []Option{{Label: "/bin/sh", Value: "/bin/sh"}}},
		},
	})
	if err != nil {
		t.Fatalf("newModel() error = %v", err)
	}
	m.list.FilterInput.SetValue("bash")
	if got := m.matchSummary(); !strings.Contains(got, "custom text") {
		t.Fatalf("matchSummary() = %q", got)
	}
}
