package steps

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// InstallSecretsStep prompts the user to supply real values for every
// kimchi:secret: placeholder that is not auto-filled by the stored Kimchi API
// key (e.g. third-party provider keys, MCP tokens).
type InstallSecretsStep struct {
	placeholders []string        // ordered list of placeholder strings
	inputs       []textinput.Model
	focused      int
	err          string
}

func NewInstallSecretsStep(placeholders []string) *InstallSecretsStep {
	inputs := make([]textinput.Model, len(placeholders))
	for i, p := range placeholders {
		ti := textinput.New()
		ti.Placeholder = "value"
		ti.EchoMode = textinput.EchoPassword
		ti.EchoCharacter = '●'
		ti.Width = 50
		ti.Prompt = fmt.Sprintf("  %s: ", Styles.Desc.Render(placeholderLabel(p)))
		inputs[i] = ti
	}
	if len(inputs) > 0 {
		inputs[0].Focus()
	}
	return &InstallSecretsStep{
		placeholders: placeholders,
		inputs:       inputs,
	}
}

// SecretValues returns a map from placeholder string → user-supplied value.
func (s *InstallSecretsStep) SecretValues() map[string]string {
	out := make(map[string]string, len(s.placeholders))
	for i, p := range s.placeholders {
		out[p] = strings.TrimSpace(s.inputs[i].Value())
	}
	return out
}

func (s *InstallSecretsStep) Init() tea.Cmd {
	return textinput.Blink
}

func (s *InstallSecretsStep) Update(msg tea.Msg) (Step, tea.Cmd) {
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
				s.inputs[s.focused].Blur()
				s.focused++
				s.inputs[s.focused].Focus()
				return s, textinput.Blink
			}
			// Validate all fields are non-empty.
			for i, inp := range s.inputs {
				if strings.TrimSpace(inp.Value()) == "" {
					s.err = fmt.Sprintf("%s is required", placeholderLabel(s.placeholders[i]))
					s.inputs[s.focused].Blur()
					s.focused = i
					s.inputs[s.focused].Focus()
					return s, textinput.Blink
				}
			}
			s.err = ""
			return s, func() tea.Msg { return NextStepMsg{} }
		}
	}

	var cmd tea.Cmd
	s.inputs[s.focused], cmd = s.inputs[s.focused].Update(msg)
	return s, cmd
}

func (s *InstallSecretsStep) View() string {
	var b strings.Builder

	b.WriteString("The recipe contains secrets from third-party providers.\n")
	b.WriteString("Enter the real values — they will be written to your local config only.\n\n")

	for i, p := range s.placeholders {
		cursor := "  "
		if s.focused == i {
			cursor = Styles.Cursor.Render("► ")
		}
		b.WriteString(cursor + Styles.Desc.Render(placeholderLabel(p)+":") + "\n")
		b.WriteString("  " + s.inputs[i].View() + "\n\n")
	}

	if s.err != "" {
		b.WriteString(Styles.Error.Render("✗ " + s.err) + "\n")
	}

	return b.String()
}

func (s *InstallSecretsStep) Name() string { return "Secrets" }

func (s *InstallSecretsStep) Info() StepInfo {
	return StepInfo{
		Name: "Enter Secrets",
		KeyBindings: []KeyBinding{
			{Key: "tab", Text: "next field"},
			BindingsConfirm,
			BindingsBack,
			BindingsQuit,
		},
	}
}

// placeholderLabel converts e.g. "kimchi:secret:OPENAI_APIKEY" → "OPENAI_APIKEY".
func placeholderLabel(placeholder string) string {
	const prefix = "kimchi:secret:"
	if after, ok := strings.CutPrefix(placeholder, prefix); ok {
		return after
	}
	return placeholder
}
