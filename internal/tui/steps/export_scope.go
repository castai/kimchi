package steps

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/config"
)

type exportScopeOption struct {
	scope config.ConfigScope
	name  string
	desc  string
}

var exportScopeOptions = []exportScopeOption{
	{
		config.ScopeGlobal,
		"Global",
		"Export your global setup (~/.config/opencode/)",
	},
	{
		config.ScopeProject,
		"Project",
		"Export the current project's config (./opencode.json)",
	},
}

// ExportScopeStep lets the user choose whether to export the global OpenCode
// config or the project-level config in the current working directory.
type ExportScopeStep struct {
	selected int
}

func NewExportScopeStep() *ExportScopeStep {
	return &ExportScopeStep{}
}

func (s *ExportScopeStep) SelectedScope() config.ConfigScope {
	return exportScopeOptions[s.selected].scope
}

func (s *ExportScopeStep) Init() tea.Cmd { return nil }

func (s *ExportScopeStep) Update(msg tea.Msg) (Step, tea.Cmd) {
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
			if s.selected < len(exportScopeOptions)-1 {
				s.selected++
			}
		case "enter":
			return s, func() tea.Msg { return NextStepMsg{} }
		}
	}
	return s, nil
}

func (s *ExportScopeStep) View() string {
	var b strings.Builder
	b.WriteString("Which OpenCode configuration do you want to export?\n\n")

	for i, opt := range exportScopeOptions {
		cursor := "  "
		if s.selected == i {
			cursor = Styles.Cursor.Render("► ")
		}
		radio := "○"
		if s.selected == i {
			radio = Styles.Selected.Render("●")
		}
		line := fmt.Sprintf("%s %s %-10s  %s", cursor, radio, opt.name, Styles.Desc.Render(opt.desc))
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func (s *ExportScopeStep) Name() string { return "Config Scope" }

func (s *ExportScopeStep) Info() StepInfo {
	return StepInfo{
		Name:        "Config Scope",
		KeyBindings: []KeyBinding{BindingsNavigate, BindingsConfirm, BindingsBack, BindingsQuit},
	}
}
