package tui

import (
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
