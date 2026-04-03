package steps

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/config"
)

type modeOption struct {
	mode        config.ConfigMode
	name        string
	description string
}

var modeOptions = []modeOption{
	{config.ModeInject, "Runtime wrapper (recommended)", "Launch via 'kimchi opencode' / 'kimchi codex'. Your existing tool configs are never modified."},
	{config.ModeOverride, "Direct override", "Write Kimchi settings into tool config files (e.g. ~/.config/opencode/opencode.json, ~/.codex/config.toml). Then run tools directly."},
}

type ModeStep struct {
	selected int
}

func NewModeStep() *ModeStep {
	return &ModeStep{selected: 0}
}

func (s *ModeStep) SelectedMode() config.ConfigMode {
	return modeOptions[s.selected].mode
}

func (s *ModeStep) Init() tea.Cmd {
	return nil
}

func (s *ModeStep) Update(msg tea.Msg) (Step, tea.Cmd) {
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
			if s.selected < len(modeOptions)-1 {
				s.selected++
			}
		case "enter":
			return s, func() tea.Msg { return NextStepMsg{} }
		}
	}
	return s, nil
}

func (s *ModeStep) View() string {
	var b strings.Builder

	b.WriteString("How should kimchi configure your tools?\n\n")

	for i, opt := range modeOptions {
		cursor := "  "
		if s.selected == i {
			cursor = Styles.Cursor.Render("► ")
		}

		radio := "○"
		if s.selected == i {
			radio = Styles.Selected.Render("●")
		}

		line := fmt.Sprintf("%s %s %s\n      %s", cursor, radio, opt.name, Styles.Desc.Render(opt.description))
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

func (s *ModeStep) Name() string {
	return "Configuration mode"
}

func (s *ModeStep) Info() StepInfo {
	return StepInfo{
		Name: "Configuration mode",
		KeyBindings: []KeyBinding{
			BindingsNavigate,
			BindingsConfirm,
			BindingsBack,
			BindingsQuit,
		},
	}
}
