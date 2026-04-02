package steps

import (
	"strings"

	"github.com/charmbracelet/bubbletea"
)

type TelemetryStep struct {
	choice   int
	selected bool
}

func NewTelemetryStep() *TelemetryStep {
	return &TelemetryStep{
		choice: 0,
	}
}

func (s *TelemetryStep) Init() tea.Cmd {
	return nil
}

func (s *TelemetryStep) Update(msg tea.Msg) (Step, tea.Cmd) {
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

func (s *TelemetryStep) View() string {
	var b strings.Builder

	b.WriteString("Help us improve your experience by sharing anonymous usage metrics.\n")
	b.WriteString("This data enhances your ")
	b.WriteString(Styles.Success.Render("Coding Report"))
	b.WriteString(" in the Kimchi console.\n\n")

	b.WriteString(Styles.Desc.Render("What we collect:"))
	b.WriteString("\n")
	b.WriteString("  • Number of requests and sessions\n")
	b.WriteString("  • Token usage and model selection\n")
	b.WriteString("  • Error rates and performance metrics\n\n")

	b.WriteString(Styles.Warning.Render("What we don't collect:"))
	b.WriteString("\n")
	b.WriteString("  • Your actual prompts or code\n")
	b.WriteString("  • File contents or sensitive data\n")
	b.WriteString("  • Personal information\n\n")

	b.WriteString(Styles.Desc.Render("───────────────────────────────────────"))
	b.WriteString("\n\n")

	optInText := "  [ ] Yes, share anonymous usage data"
	optOutText := "  [ ] No, keep my usage private"

	if s.choice == 0 {
		optInText = Styles.Cursor.Render("► ") + Styles.Selected.Render("[✓] Yes, share anonymous usage data")
		b.WriteString(optInText)
		b.WriteString("\n")
		b.WriteString(optOutText)
	} else {
		b.WriteString(optInText)
		b.WriteString("\n")
		optOutText = Styles.Cursor.Render("► ") + Styles.Selected.Render("[✓] No, keep my usage private")
		b.WriteString(optOutText)
	}

	return b.String()
}

func (s *TelemetryStep) Name() string {
	return "Telemetry"
}

func (s *TelemetryStep) Info() StepInfo {
	return StepInfo{
		Name: "Usage Telemetry",
		KeyBindings: []KeyBinding{
			BindingsNavigate,
			BindingsConfirm,
			BindingsBack,
			BindingsQuit,
		},
	}
}

func (s *TelemetryStep) OptIn() bool {
	return s.choice == 0
}
