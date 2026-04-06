package steps

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/recipe"
)

type installSourceState int

const (
	installSourceIdle installSourceState = iota
	installSourceParsing
	installSourceValid
	installSourceInvalid
)

type parseCompleteMsg struct {
	r   *recipe.Recipe
	err error
}

type installSourceAdvanceMsg struct{}

// InstallSourceStep collects the recipe source and resolves it asynchronously.
// Source may be a local file path, a recipe name, "cookbook/name", or "name@version".
type InstallSourceStep struct {
	input     textinput.Model
	spin      spinner.Model
	state     installSourceState
	parsed    *recipe.Recipe
	errMsg    string
	autoStart bool
}

func NewInstallSourceStep(prefillSource string) *InstallSourceStep {
	ti := textinput.New()
	ti.Placeholder = "path/to/recipe.yaml  or  name  or  name@version"
	ti.Width = 60
	if prefillSource != "" {
		ti.SetValue(prefillSource)
	}
	ti.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return &InstallSourceStep{
		input:     ti,
		spin:      sp,
		state:     installSourceIdle,
		autoStart: prefillSource != "",
	}
}

func (s *InstallSourceStep) ParsedRecipe() *recipe.Recipe { return s.parsed }

func (s *InstallSourceStep) Init() tea.Cmd {
	if s.autoStart {
		return tea.Batch(s.spin.Tick, s.resolve(s.input.Value()))
	}
	return textinput.Blink
}

func (s *InstallSourceStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return s, func() tea.Msg { return AbortMsg{} }
		case "esc":
			if s.state != installSourceParsing {
				return s, func() tea.Msg { return PrevStepMsg{} }
			}
		case "enter":
			if s.state == installSourceIdle || s.state == installSourceInvalid {
				src := strings.TrimSpace(s.input.Value())
				if src == "" {
					s.errMsg = "Recipe source is required"
					s.state = installSourceInvalid
					return s, nil
				}
				s.state = installSourceParsing
				s.errMsg = ""
				return s, tea.Batch(s.spin.Tick, s.resolve(src))
			}
			if s.state == installSourceValid {
				return s, func() tea.Msg { return NextStepMsg{} }
			}
		}

	case parseCompleteMsg:
		if msg.err != nil {
			s.state = installSourceInvalid
			s.errMsg = msg.err.Error()
			return s, nil
		}
		s.parsed = msg.r
		s.state = installSourceValid
		return s, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return installSourceAdvanceMsg{}
		})

	case installSourceAdvanceMsg:
		return s, func() tea.Msg { return NextStepMsg{} }

	case spinner.TickMsg:
		if s.state == installSourceParsing {
			var cmd tea.Cmd
			s.spin, cmd = s.spin.Update(msg)
			return s, cmd
		}
	}

	if s.state == installSourceIdle || s.state == installSourceInvalid {
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		return s, cmd
	}
	return s, nil
}

func (s *InstallSourceStep) resolve(source string) tea.Cmd {
	return func() tea.Msg {
		r, err := recipe.ResolveSource(source)
		return parseCompleteMsg{r: r, err: err}
	}
}

func (s *InstallSourceStep) View() string {
	var b strings.Builder

	b.WriteString("Enter the recipe to install.\n")
	b.WriteString(Styles.Desc.Render("Supported: file path · name · cookbook/name · name@version") + "\n\n")
	b.WriteString(Styles.Desc.Render("Source:"))
	b.WriteString("\n")
	b.WriteString(s.input.View())
	b.WriteString("\n\n")

	switch s.state {
	case installSourceParsing:
		b.WriteString(Styles.Spinner.Render(fmt.Sprintf("%s Resolving recipe…", s.spin.View())))
	case installSourceValid:
		b.WriteString(Styles.Success.Render(fmt.Sprintf("✓ Recipe \"%s\" (%s) loaded", s.parsed.Name, s.parsed.Version)))
	case installSourceInvalid:
		b.WriteString(Styles.Error.Render(fmt.Sprintf("✗ %s", s.errMsg)))
	}
	b.WriteString("\n")

	return b.String()
}

func (s *InstallSourceStep) Name() string { return "Recipe Source" }

func (s *InstallSourceStep) Info() StepInfo {
	return StepInfo{
		Name:        "Recipe Source",
		KeyBindings: []KeyBinding{BindingsConfirm, BindingsBack, BindingsQuit},
	}
}
