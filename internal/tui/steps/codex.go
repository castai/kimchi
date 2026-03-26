package steps

import (
	"strings"

	"github.com/charmbracelet/bubbletea"
)

type CodexStep struct {
	choice   int
	selected bool
}

func NewCodexStep() *CodexStep {
	return &CodexStep{
		choice: 0,
	}
}

func (s *CodexStep) Init() tea.Cmd {
	return nil
}

func (s *CodexStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return s, func() tea.Msg { return AbortMsg{} }
		case "esc":
			return s, func() tea.Msg { return PrevStepMsg{} }
		case "up", "k", "down", "j":
			if s.choice == 0 {
				s.choice = 1
			} else {
				s.choice = 0
			}
		case "enter":
			s.selected = true
			return s, func() tea.Msg { return NextStepMsg{} }
		}
	}
	return s, nil
}

func (s *CodexStep) View() string {
	var b strings.Builder

	b.WriteString("Set Kimchi as the default model provider for Codex?\n\n")

	yesText := "  [ ] Yes"
	noText := "  [ ] No"

	if s.choice == 0 {
		yesText = Styles.Cursor.Render("► ") + Styles.Selected.Render("[✓] Yes")
		b.WriteString(yesText)
		b.WriteString("\n")
		b.WriteString(noText)
	} else {
		b.WriteString(yesText)
		b.WriteString("\n")
		noText = Styles.Cursor.Render("► ") + Styles.Selected.Render("[✓] No")
		b.WriteString(noText)
	}

	return b.String()
}

func (s *CodexStep) Name() string {
	return "Codex"
}

func (s *CodexStep) Info() StepInfo {
	return StepInfo{
		Name: "Codex Configuration",
		KeyBindings: []KeyBinding{
			BindingsNavigate,
			BindingsConfirm,
			BindingsBack,
			BindingsQuit,
		},
	}
}

func (s *CodexStep) SetAsDefault() bool {
	return s.choice == 0
}
