package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Option struct {
	Label       string
	Details     string
	Value       string
	Meta        map[string]string
}

func (o Option) FilterValue() string { return o.Label + " " + o.Details + " " + o.Value }
func (o Option) TitleString() string { return o.Label }
func (o Option) DescriptionString() string { return o.Details }

func (o Option) Title() string       { return o.Label }
func (o Option) Description() string { return o.Details }

type Step struct {
	Key          string
	Title        string
	Placeholder  string
	Options      []Option
	Load         func(state WorkflowState) ([]Option, error)
	AllowCustom  bool
	DefaultValue string
}

type WorkflowState struct {
	Profile   string
	Region    string
	Cluster   string
	Service   string
	Task      string
	Instance  string
	Container string
	Command   string
}

type WorkflowInput struct {
	Title string
	Steps []Step
	State WorkflowState
}

type WorkflowOutput struct {
	State WorkflowState
}

type keyMap struct {
	Select key.Binding
	Back   key.Binding
	Quit   key.Binding
}

func defaultKeys() keyMap {
	return keyMap{
		Select: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
		Back:   key.NewBinding(key.WithKeys("esc", "backspace"), key.WithHelp("esc", "back")),
		Quit:   key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
	}
}

type model struct {
	title   string
	steps   []Step
	index   int
	list    list.Model
	keys    keyMap
	help    help.Model
	state   WorkflowState
	width   int
	height  int
	err     error
	done    bool
	quitting bool
	options []Option
}

func RunWorkflow(ctx context.Context, input WorkflowInput) (WorkflowOutput, error) {
	m, err := newModel(input)
	if err != nil {
		return WorkflowOutput{}, err
	}

	program := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx))
	result, err := program.Run()
	if err != nil {
		return WorkflowOutput{}, err
	}

	finalModel, ok := result.(model)
	if !ok {
		return WorkflowOutput{}, fmt.Errorf("unexpected tui model type %T", result)
	}
	if finalModel.quitting {
		return WorkflowOutput{}, tea.ErrProgramKilled
	}
	if finalModel.err != nil {
		return WorkflowOutput{}, finalModel.err
	}

	return WorkflowOutput{State: finalModel.state}, nil
}

func newModel(input WorkflowInput) (model, error) {
	if len(input.Steps) == 0 {
		return model{}, fmt.Errorf("workflow requires at least one step")
	}

	m := model{
		title: input.Title,
		steps: input.Steps,
		keys:  defaultKeys(),
		help:  help.New(),
		state: input.State,
	}

	options, err := loadStepOptions(input.Steps[0], input.State)
	if err != nil {
		return model{}, err
	}
	if len(options) == 0 {
		return model{}, fmt.Errorf("%s has no available options", input.Steps[0].Title)
	}

	l := list.New(toListItems(options), optionDelegate{}, 0, 0)
	l.Title = input.Steps[0].Title
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	if input.Steps[0].DefaultValue != "" {
		index := findOptionIndex(options, input.Steps[0].DefaultValue)
		if index >= 0 {
			l.Select(index)
		}
	}
	m.list = l
	m.options = options

	return m, nil
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = typed.Width
		m.height = typed.Height
		m.list.SetSize(typed.Width, max(typed.Height-8, 8))
		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(typed, m.keys.Quit):
			m.quitting = true
			return m, tea.Quit
		case key.Matches(typed, m.keys.Back):
			if m.index > 0 {
				m.index--
				if err := m.setStep(m.index); err != nil {
					m.err = err
					return m, tea.Quit
				}
				return m, nil
			}
		case key.Matches(typed, m.keys.Select):
			if selected, ok := m.list.SelectedItem().(Option); ok {
				m.applySelection(selected)
				if m.index == len(m.steps)-1 {
					m.done = true
					return m, tea.Quit
				}
				m.index++
				if err := m.setStep(m.index); err != nil {
					m.err = err
					return m, tea.Quit
				}
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.done {
		return ""
	}

	status := renderStatus(m.state)
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(m.help.ShortHelpView([]key.Binding{
		m.keys.Select,
		m.keys.Back,
		m.keys.Quit,
	}))

	body := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Render(m.title),
		status,
		m.list.View(),
		footer,
	)

	return lipgloss.NewStyle().Padding(1, 2).Render(body)
}

func renderStatus(state WorkflowState) string {
	parts := []string{
		renderTag("profile", emptyFallback(state.Profile)),
		renderTag("region", emptyFallback(state.Region)),
	}
	if state.Cluster != "" || state.Service != "" {
		parts = append(parts, renderTag("cluster", emptyFallback(state.Cluster)))
		parts = append(parts, renderTag("service", emptyFallback(state.Service)))
	}
	if state.Instance != "" {
		parts = append(parts, renderTag("instance", emptyFallback(state.Instance)))
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, parts...)
}

func renderTag(label, value string) string {
	return lipgloss.NewStyle().
		MarginRight(1).
		Padding(0, 1).
		Background(lipgloss.Color("238")).
		Foreground(lipgloss.Color("230")).
		Render(fmt.Sprintf("%s: %s", label, value))
}

func emptyFallback(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func (m *model) applySelection(option Option) {
	switch m.steps[m.index].Key {
	case "profile":
		m.state.Profile = option.Value
	case "region":
		m.state.Region = option.Value
	case "cluster":
		m.state.Cluster = option.Value
	case "service":
		m.state.Service = option.Value
	case "task":
		m.state.Task = option.Value
	case "container":
		m.state.Container = option.Value
	case "instance":
		m.state.Instance = option.Value
	case "command":
		m.state.Command = option.Value
	}
}

func (m *model) setStep(index int) error {
	options, err := loadStepOptions(m.steps[index], m.state)
	if err != nil {
		return err
	}
	if len(options) == 0 {
		return fmt.Errorf("%s has no available options", m.steps[index].Title)
	}

	m.options = options
	m.list.Title = m.steps[index].Title
	m.list.SetItems(toListItems(options))
	m.list.Select(0)
	if m.steps[index].DefaultValue != "" {
		defaultIndex := findOptionIndex(options, m.steps[index].DefaultValue)
		if defaultIndex >= 0 {
			m.list.Select(defaultIndex)
		}
	}
	return nil
}

func toListItems(options []Option) []list.Item {
	items := make([]list.Item, 0, len(options))
	for _, option := range options {
		items = append(items, option)
	}
	return items
}

func findOptionIndex(options []Option, value string) int {
	for i, option := range options {
		if option.Value == value {
			return i
		}
	}
	return -1
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func loadStepOptions(step Step, state WorkflowState) ([]Option, error) {
	if step.Load != nil {
		return step.Load(state)
	}
	return step.Options, nil
}
