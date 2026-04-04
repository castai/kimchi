package steps

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type useCaseOption struct {
	value string
	label string
	desc  string
}

var useCaseOptions = []useCaseOption{
	{"coding", "Coding", "Focused on code generation and debugging"},
	{"research", "Research", "Deep reasoning and analysis tasks"},
	{"balanced", "Balanced", "Mix of coding and reasoning"},
	{"custom", "Custom", "Define your own use case"},
}

// ExportUseCaseStep is a radio-select step for the recipe's use_case tag.
type ExportUseCaseStep struct {
	selected int
}

func NewExportUseCaseStep() *ExportUseCaseStep {
	return &ExportUseCaseStep{selected: 0}
}

func (s *ExportUseCaseStep) SelectedUseCase() string {
	return useCaseOptions[s.selected].value
}

func (s *ExportUseCaseStep) Init() tea.Cmd { return nil }

func (s *ExportUseCaseStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return s, func() tea.Msg { return AbortMsg{} }
		case "esc":
			return s, func() tea.Msg { return PrevStepMsg{} }
		case "up", "k":
			if s.selected > 0 {
				s.selected--
			}
		case "down", "j":
			if s.selected < len(useCaseOptions)-1 {
				s.selected++
			}
		case "enter":
			return s, func() tea.Msg { return NextStepMsg{} }
		}
	}
	return s, nil
}

func (s *ExportUseCaseStep) View() string {
	var b strings.Builder

	b.WriteString("How will this recipe primarily be used?\n\n")

	for i, opt := range useCaseOptions {
		cursor := "  "
		if s.selected == i {
			cursor = Styles.Cursor.Render("► ")
		}
		radio := "○"
		if s.selected == i {
			radio = Styles.Selected.Render("●")
		}
		line := fmt.Sprintf("%s %s %-10s  %s", cursor, radio, opt.label, Styles.Desc.Render(opt.desc))
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

func (s *ExportUseCaseStep) Name() string { return "Use Case" }

func (s *ExportUseCaseStep) Info() StepInfo {
	return StepInfo{
		Name: "Use Case",
		KeyBindings: []KeyBinding{
			BindingsNavigate,
			BindingsConfirm,
			BindingsBack,
			BindingsQuit,
		},
	}
}
