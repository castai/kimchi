package steps

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ExportOutputStep asks the user where to write the recipe file.
// The input is pre-filled with defaultPath; pressing Enter without editing accepts it.
type ExportOutputStep struct {
	input textinput.Model
}

func NewExportOutputStep(defaultPath string) *ExportOutputStep {
	ti := textinput.New()
	ti.Placeholder = "kimchi-recipe.yaml"
	ti.SetValue(defaultPath)
	ti.Width = 60
	ti.Focus()
	return &ExportOutputStep{input: ti}
}

// OutputPath returns the chosen file path (trimmed).
func (s *ExportOutputStep) OutputPath() string {
	v := strings.TrimSpace(s.input.Value())
	if v == "" {
		return s.input.Placeholder
	}
	return v
}

func (s *ExportOutputStep) Init() tea.Cmd { return textinput.Blink }

func (s *ExportOutputStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return s, func() tea.Msg { return AbortMsg{} }
		case "esc":
			return s, func() tea.Msg { return PrevStepMsg{} }
		case "enter":
			return s, func() tea.Msg { return NextStepMsg{} }
		}
	}
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return s, cmd
}

func (s *ExportOutputStep) View() string {
	var b strings.Builder
	b.WriteString("Where should the recipe file be saved?\n\n")
	b.WriteString(Styles.Desc.Render("Output file:"))
	b.WriteString("\n")
	b.WriteString(s.input.View())
	b.WriteString("\n\n")
	b.WriteString(Styles.Desc.Render("Press enter to accept, or type a new path."))
	b.WriteString("\n")
	return b.String()
}

func (s *ExportOutputStep) Name() string { return "Output File" }

func (s *ExportOutputStep) Info() StepInfo {
	return StepInfo{
		Name:        "Output File",
		KeyBindings: []KeyBinding{BindingsConfirm, BindingsBack, BindingsQuit},
	}
}
