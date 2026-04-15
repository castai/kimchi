package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/castai/kimchi/internal/tools"
)

type modelRolesPhase int

const (
	modelRolesPhaseLoading modelRolesPhase = iota
	modelRolesPhaseMain
	modelRolesPhaseCoding
	modelRolesPhaseSub
	modelRolesPhaseDone
	modelRolesPhaseError
)

type modelsLoadedMsg struct {
	models []tools.Model
	err    error
}

var modelRolesSpinStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))

// ModelRolesStep fetches the available models and lets the user assign the
// Main, Coding, and Sub roles interactively before the configure step.
type ModelRolesStep struct {
	apiKey string
	phase  modelRolesPhase
	models []tools.Model
	cursor int
	errMsg string
	spin   spinner.Model

	mainModel   tools.Model
	codingModel tools.Model
	subModel    tools.Model
}

func NewModelRolesStep(apiKey string) *ModelRolesStep {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = modelRolesSpinStyle

	return &ModelRolesStep{
		apiKey: apiKey,
		phase:  modelRolesPhaseLoading,
		spin:   sp,
	}
}

// ModelConfig returns the assigned roles once the step is complete.
func (s *ModelRolesStep) ModelConfig() tools.ModelConfig {
	return tools.ModelConfig{
		Main:   s.mainModel,
		Coding: s.codingModel,
		Sub:    s.subModel,
		All:    s.models,
	}
}

func (s *ModelRolesStep) Init() tea.Cmd {
	return tea.Batch(s.spin.Tick, s.fetchModels())
}

func (s *ModelRolesStep) fetchModels() tea.Cmd {
	return func() tea.Msg {
		models, err := tools.FetchModels(context.Background(), s.apiKey)
		return modelsLoadedMsg{models: models, err: err}
	}
}

func (s *ModelRolesStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case modelsLoadedMsg:
		if msg.err != nil {
			s.phase = modelRolesPhaseError
			s.errMsg = msg.err.Error()
			return s, nil
		}
		if len(msg.models) == 0 {
			s.phase = modelRolesPhaseError
			s.errMsg = "no models returned from API"
			return s, nil
		}
		s.models = msg.models
		// Pre-assign defaults using the well-known slugs. BuildModelConfig falls
		// back positionally when a slug is not found in the list.
		defaults := tools.BuildModelConfig(msg.models, "kimi-k2.5", "nemotron-3-super-fp4", "minimax-m2.7")
		s.mainModel = defaults.Main
		s.codingModel = defaults.Coding
		s.subModel = defaults.Sub
		s.phase = modelRolesPhaseMain
		s.cursor = indexOf(msg.models, defaults.Main)
		return s, nil

	case spinner.TickMsg:
		if s.phase == modelRolesPhaseLoading {
			var cmd tea.Cmd
			s.spin, cmd = s.spin.Update(msg)
			return s, cmd
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return s, func() tea.Msg { return AbortMsg{} }
		case "esc":
			switch s.phase {
			case modelRolesPhaseMain:
				return s, func() tea.Msg { return PrevStepMsg{} }
			case modelRolesPhaseCoding:
				s.phase = modelRolesPhaseMain
				s.cursor = indexOf(s.models, s.mainModel)
			case modelRolesPhaseSub:
				s.phase = modelRolesPhaseCoding
				s.cursor = indexOf(s.models, s.codingModel)
			}
			return s, nil
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < len(s.models)-1 {
				s.cursor++
			}
		case "enter":
			switch s.phase {
			case modelRolesPhaseMain:
				s.mainModel = s.models[s.cursor]
				s.phase = modelRolesPhaseCoding
				s.cursor = indexOf(s.models, s.codingModel)
			case modelRolesPhaseCoding:
				s.codingModel = s.models[s.cursor]
				s.phase = modelRolesPhaseSub
				s.cursor = indexOf(s.models, s.subModel)
			case modelRolesPhaseSub:
				s.subModel = s.models[s.cursor]
				s.phase = modelRolesPhaseDone
				return s, func() tea.Msg { return NextStepMsg{} }
			case modelRolesPhaseError:
				return s, func() tea.Msg { return AbortMsg{} }
			}
		}
	}

	return s, nil
}

func (s *ModelRolesStep) View() string {
	var b strings.Builder

	switch s.phase {
	case modelRolesPhaseLoading:
		b.WriteString(fmt.Sprintf("%s Fetching available models...", s.spin.View()))
		return b.String()

	case modelRolesPhaseError:
		b.WriteString(Styles.Error.Render(fmt.Sprintf("✗ Failed to fetch models: %s\n\n", s.errMsg)))
		b.WriteString(Styles.Help.Render("Press Enter to abort"))
		return b.String()

	case modelRolesPhaseMain:
		b.WriteString("Select the ")
		b.WriteString(Styles.Selected.Render("Main"))
		b.WriteString(" model (reasoning, planning, image processing):\n\n")
		s.renderModelList(&b)

	case modelRolesPhaseCoding:
		b.WriteString(fmt.Sprintf("Main: %s\n\n", Styles.Success.Render(s.mainModel.Slug)))
		b.WriteString("Select the ")
		b.WriteString(Styles.Selected.Render("Coding"))
		b.WriteString(" model (code generation and debugging):\n\n")
		s.renderModelList(&b)

	case modelRolesPhaseSub:
		b.WriteString(fmt.Sprintf("Main: %s   Coding: %s\n\n",
			Styles.Success.Render(s.mainModel.Slug),
			Styles.Success.Render(s.codingModel.Slug)))
		b.WriteString("Select the ")
		b.WriteString(Styles.Selected.Render("Sub"))
		b.WriteString(" model (secondary subagent):\n\n")
		s.renderModelList(&b)

	case modelRolesPhaseDone:
		b.WriteString(Styles.Success.Render("✓ Model roles assigned"))
	}

	return b.String()
}

func (s *ModelRolesStep) renderModelList(b *strings.Builder) {
	for i, m := range s.models {
		cursor := "  "
		if s.cursor == i {
			cursor = Styles.Cursor.Render("► ")
		}

		radio := "○"
		if s.cursor == i {
			radio = Styles.Selected.Render("●")
		}

		desc := m.DisplayName
		if m.Description != "" {
			desc += " — " + m.Description
		}

		line := fmt.Sprintf("%s %s %s\n", cursor, radio, m.Slug)
		b.WriteString(line)
		if desc != "" {
			b.WriteString(fmt.Sprintf("       %s\n", Styles.Desc.Render(desc)))
		}
	}
}

func (s *ModelRolesStep) Name() string {
	return "Model roles"
}

func (s *ModelRolesStep) Info() StepInfo {
	bindings := []KeyBinding{BindingsNavigate, BindingsConfirm, BindingsBack, BindingsQuit}
	if s.phase == modelRolesPhaseLoading {
		bindings = nil
	}
	return StepInfo{
		Name:        "Assign model roles",
		KeyBindings: bindings,
	}
}

func indexOf(models []tools.Model, m tools.Model) int {
	for i, model := range models {
		if model.Slug == m.Slug {
			return i
		}
	}
	return 0
}
