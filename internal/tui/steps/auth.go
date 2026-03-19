package steps

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/auth"
	"github.com/castai/kimchi/internal/config"
)

type authState int

const (
	authStateInput authState = iota
	authStateValidating
	authStateValid
	authStateInvalid
)

type validationCompleteMsg struct {
	valid       bool
	errMsg      string
	suggestions []string
}

type advanceMsg struct{}

// AuthStep is the API key input and validation wizard step.
type AuthStep struct {
	input     textinput.Model
	spin      spinner.Model
	state     authState
	errMsg    string
	validator auth.Validator
}

func NewAuthStep() *AuthStep {
	ti := textinput.New()
	ti.Placeholder = "Enter your Cast AI API key"
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '●'
	ti.Width = 50
	ti.Focus()

	if key, _ := config.GetAPIKey(); key != "" {
		ti.SetValue(key)
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return &AuthStep{
		input:     ti,
		spin:      sp,
		state:     authStateInput,
		validator: auth.NewValidator(nil),
	}
}

func (s *AuthStep) APIKey() string {
	return strings.TrimSpace(s.input.Value())
}

func (s *AuthStep) Init() tea.Cmd {
	return textinput.Blink
}

func (s *AuthStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return s, tea.Batch(func() tea.Msg { return AbortMsg{} })
		case "esc":
			return s, func() tea.Msg { return PrevStepMsg{} }
		case "enter":
			switch s.state {
			case authStateInput, authStateInvalid:
				apiKey := strings.TrimSpace(s.input.Value())
				if apiKey == "" {
					s.state = authStateInvalid
					s.errMsg = "API key is required"
					return s, nil
				}
				s.state = authStateValidating
				return s, tea.Batch(s.spin.Tick, s.validate(apiKey))
			case authStateValid:
				return s, func() tea.Msg { return NextStepMsg{} }
			}
		}

	case validationCompleteMsg:
		if msg.valid {
			s.state = authStateValid
			if err := config.SetAPIKey(strings.TrimSpace(s.input.Value())); err != nil {
				s.state = authStateInvalid
				s.errMsg = fmt.Sprintf("Failed to save API key: %v", err)
				return s, nil
			}
			return s, tea.Tick(800*time.Millisecond, func(t time.Time) tea.Msg {
				return advanceMsg{}
			})
		}
		s.state = authStateInvalid
		s.errMsg = msg.errMsg
		return s, nil

	case advanceMsg:
		return s, func() tea.Msg { return NextStepMsg{} }

	case spinner.TickMsg:
		if s.state == authStateValidating {
			var cmd tea.Cmd
			s.spin, cmd = s.spin.Update(msg)
			return s, cmd
		}
	}

	if s.state == authStateInput || s.state == authStateInvalid {
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		return s, cmd
	}

	return s, nil
}

func (s *AuthStep) validate(apiKey string) tea.Cmd {
	return func() tea.Msg {
		result, err := s.validator.ValidateAPIKey(context.Background(), apiKey)
		if err != nil {
			return validationCompleteMsg{valid: false, errMsg: err.Error()}
		}
		return validationCompleteMsg{
			valid:       result.Valid,
			errMsg:      result.Error,
			suggestions: result.Suggestions,
		}
	}
}

func (s *AuthStep) View() string {
	var b strings.Builder

	b.WriteString(Styles.Title.Render("Cast AI API Key"))
	b.WriteString("\n\n")

	b.WriteString("You need an API key to use Cast AI's open-source models.\n")
	b.WriteString("To create one:\n\n")

	b.WriteString("  1. Open ")
	b.WriteString(Hyperlink("https://console.cast.ai/user/api-access", "https://console.cast.ai/user/api-access"))
	b.WriteString("\n")
	b.WriteString("  2. Click \"Create access key\"\n")
	b.WriteString("  3. Paste the key below\n\n")

	b.WriteString("You'll be prompted to log in if you don't have an account.\n\n")

	b.WriteString(Styles.Desc.Render("API Key:"))
	b.WriteString("\n")
	b.WriteString(s.input.View())
	b.WriteString("\n\n")

	switch s.state {
	case authStateValidating:
		b.WriteString(Styles.Spinner.Render(fmt.Sprintf("%s Validating...", s.spin.View())))
		b.WriteString("\n")
	case authStateValid:
		b.WriteString(Styles.Success.Render("✓ API key validated successfully"))
		b.WriteString("\n")
	case authStateInvalid:
		b.WriteString(Styles.Error.Render(fmt.Sprintf("✗ %s", s.errMsg)))
		b.WriteString("\n")
	}

	return b.String()
}

func (s *AuthStep) Name() string {
	return "Auth"
}

func (s *AuthStep) Info() StepInfo {
	return StepInfo{
		Name: "Auth",
		KeyBindings: []KeyBinding{
			BindingsConfirm,
			BindingsBack,
			BindingsQuit,
		},
	}
}
