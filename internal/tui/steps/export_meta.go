package steps

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ExportMetaStep collects recipe metadata: name, author, description.
type ExportMetaStep struct {
	inputs  [3]textinput.Model
	focused int
	err     string
}

// NewExportMetaStep creates the metadata step. initialName pre-fills the name
// field when provided via the --name flag, so the wizard can skip the prompt.
func NewExportMetaStep(initialName string) *ExportMetaStep {
	labels := []string{"Recipe name", "Author", "Description (optional)"}
	s := &ExportMetaStep{}
	for i, label := range labels {
		ti := textinput.New()
		ti.Placeholder = label
		ti.Width = 50
		s.inputs[i] = ti
	}
	if initialName != "" {
		s.inputs[0].SetValue(initialName)
	}
	s.inputs[0].Focus()
	return s
}

func (s *ExportMetaStep) RecipeName() string   { return strings.TrimSpace(s.inputs[0].Value()) }
func (s *ExportMetaStep) Author() string        { return strings.TrimSpace(s.inputs[1].Value()) }
func (s *ExportMetaStep) Description() string   { return strings.TrimSpace(s.inputs[2].Value()) }

func (s *ExportMetaStep) Init() tea.Cmd {
	return textinput.Blink
}

func (s *ExportMetaStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return s, func() tea.Msg { return AbortMsg{} }
		case "esc":
			if s.focused > 0 {
				s.inputs[s.focused].Blur()
				s.focused--
				s.inputs[s.focused].Focus()
				return s, textinput.Blink
			}
			return s, func() tea.Msg { return PrevStepMsg{} }
		case "tab", "down":
			s.inputs[s.focused].Blur()
			s.focused = (s.focused + 1) % len(s.inputs)
			s.inputs[s.focused].Focus()
			return s, textinput.Blink
		case "shift+tab", "up":
			s.inputs[s.focused].Blur()
			s.focused = (s.focused - 1 + len(s.inputs)) % len(s.inputs)
			s.inputs[s.focused].Focus()
			return s, textinput.Blink
		case "enter":
			if s.focused < len(s.inputs)-1 {
				// Advance to next field.
				s.inputs[s.focused].Blur()
				s.focused++
				s.inputs[s.focused].Focus()
				return s, textinput.Blink
			}
			// Last field — validate required fields and advance step.
			if s.RecipeName() == "" {
				s.err = "Recipe name is required"
				s.inputs[s.focused].Blur()
				s.focused = 0
				s.inputs[s.focused].Focus()
				return s, textinput.Blink
			}
			if s.Author() == "" {
				s.err = "Author is required"
				s.inputs[s.focused].Blur()
				s.focused = 1
				s.inputs[s.focused].Focus()
				return s, textinput.Blink
			}
			s.err = ""
			return s, func() tea.Msg { return NextStepMsg{} }
		}
	}

	var cmd tea.Cmd
	s.inputs[s.focused], cmd = s.inputs[s.focused].Update(msg)
	return s, cmd
}

func (s *ExportMetaStep) View() string {
	var b strings.Builder

	b.WriteString("Provide metadata for the recipe file.\n\n")

	labels := []string{"Name", "Author", "Description"}
	for i, label := range labels {
		cursor := "  "
		if s.focused == i {
			cursor = Styles.Cursor.Render("► ")
		}
		b.WriteString(cursor + Styles.Desc.Render(label+":\n"))
		b.WriteString("  " + s.inputs[i].View())
		b.WriteString("\n\n")
	}

	if s.err != "" {
		b.WriteString(Styles.Error.Render("✗ " + s.err))
		b.WriteString("\n")
	}

	return b.String()
}

func (s *ExportMetaStep) Name() string { return "Recipe Metadata" }

func (s *ExportMetaStep) Info() StepInfo {
	return StepInfo{
		Name: "Recipe Metadata",
		KeyBindings: []KeyBinding{
			{Key: "tab", Text: "next field"},
			BindingsConfirm,
			BindingsBack,
			BindingsQuit,
		},
	}
}
