package steps

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/auth"
	"github.com/castai/kimchi/internal/config"
)

type authState int

const (
	authStateInput authState = iota
	authStateValidating
	authStateValid
	authStateInvalid
	authStateSaved
	authStateValidatingSaved
	authStateValidSaved
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
	savedKey  string
	validator auth.Validator
}

func NewAuthStep() *AuthStep {
	ti := textinput.New()
	ti.Placeholder = "Enter your Kimchi API key"
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '●'
	// Width is intentionally left at the default: a fixed cap scrolls the
	// masked viewport on backspace instead of shrinking visible dots, which
	// reads as an unresponsive terminal when editing long pasted keys.
	ti.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return &AuthStep{
		input:     ti,
		spin:      sp,
		validator: auth.NewValidator(nil),
	}
}

func (s *AuthStep) APIKey() string {
	return s.activeKey()
}

func (s *AuthStep) activeKey() string {
	if s.inSavedFrame() {
		return s.savedKey
	}
	return strings.TrimSpace(s.input.Value())
}

func (s *AuthStep) Init() tea.Cmd {
	// Re-entering the step (via back/forward) snaps back to the saved view so
	// the user gets a fresh chance to keep or replace the stored key instead
	// of resuming a half-typed edit.
	if key, _ := config.GetAPIKey(); key != "" {
		s.savedKey = key
		s.state = authStateSaved
		s.errMsg = ""
	}
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
			case authStateSaved:
				s.state = authStateValidatingSaved
				return s, tea.Batch(s.spin.Tick, s.validate(s.savedKey))
			case authStateInput, authStateInvalid:
				apiKey := strings.TrimSpace(s.input.Value())
				if apiKey == "" {
					s.state = authStateInvalid
					s.errMsg = "API key is required"
					return s, nil
				}
				s.state = authStateValidating
				return s, tea.Batch(s.spin.Tick, s.validate(apiKey))
			case authStateValid, authStateValidSaved:
				return s, func() tea.Msg { return NextStepMsg{} }
			}
		case "y", "Y":
			if s.state == authStateSaved {
				s.state = authStateValidatingSaved
				return s, tea.Batch(s.spin.Tick, s.validate(s.savedKey))
			}
		case "n", "N":
			if s.state == authStateSaved {
				s.state = authStateInput
				return s, nil
			}
		}

	case validationCompleteMsg:
		fromSaved := s.state == authStateValidatingSaved
		if s.state != authStateValidating && !fromSaved {
			return s, nil
		}
		if msg.valid {
			if fromSaved {
				s.state = authStateValidSaved
			} else {
				s.state = authStateValid
				if err := config.SetAPIKey(s.activeKey()); err != nil {
					s.state = authStateInvalid
					s.errMsg = fmt.Sprintf("Failed to save API key: %v", err)
					return s, nil
				}
			}
			return s, tea.Tick(800*time.Millisecond, func(t time.Time) tea.Msg {
				return advanceMsg{}
			})
		}
		s.state = authStateInvalid
		s.errMsg = msg.errMsg
		return s, nil

	case advanceMsg:
		if s.state != authStateValid && s.state != authStateValidSaved {
			return s, nil
		}
		return s, func() tea.Msg { return NextStepMsg{} }

	case spinner.TickMsg:
		if s.state == authStateValidating || s.state == authStateValidatingSaved {
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

func (s *AuthStep) inSavedFrame() bool {
	switch s.state {
	case authStateSaved, authStateValidatingSaved, authStateValidSaved:
		return true
	}
	return false
}

func (s *AuthStep) View() string {
	var b strings.Builder

	if s.inSavedFrame() {
		b.WriteString("An API key is already saved for Kimchi.\n")
		if s.state == authStateSaved {
			b.WriteString("\n")
			b.WriteString("Use the saved API key? [Y/n]\n")
		}
		switch s.state {
		case authStateValidatingSaved:
			b.WriteString("\n")
			b.WriteString(Styles.Spinner.Render(fmt.Sprintf("%s Validating saved API key...", s.spin.View())))
			b.WriteString("\n")
		case authStateValidSaved:
			b.WriteString("\n")
			b.WriteString(Styles.Success.Render("✓ API key validated successfully"))
			b.WriteString("\n")
		}
		return b.String()
	}

	b.WriteString("You need an API key to use Kimchi's open-source models.\n")
	b.WriteString("To create one:\n\n")

	b.WriteString("  1. Open ")
	b.WriteString(Hyperlink("https://app.kimchi.dev", "https://app.kimchi.dev"))
	b.WriteString("\n")
	b.WriteString("  2. Go to API Keys → Create API Key\n")
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
	var bindings []KeyBinding
	if s.state == authStateSaved {
		bindings = []KeyBinding{
			{Key: "Y", Text: "use saved"},
			{Key: "n", Text: "change"},
			BindingsConfirm,
		}
	} else {
		bindings = []KeyBinding{BindingsConfirm}
	}
	bindings = append(bindings, BindingsBack, BindingsQuit)
	return StepInfo{
		Name:        "Kimchi API Key",
		KeyBindings: bindings,
	}
}
