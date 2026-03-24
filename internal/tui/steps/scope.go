package steps

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/config"
)

type scopeOption struct {
	scope       config.ConfigScope
	name        string
	description string
}

var scopeOptions = []scopeOption{
	{config.ScopeGlobal, "Global", "Configure for all projects (~/.config/...)"},
	{config.ScopeProject, "Project", "Configure for current directory only"},
}

type ScopeStep struct {
	selected int
}

func NewScopeStep() *ScopeStep {
	return &ScopeStep{selected: 0}
}

func (s *ScopeStep) SelectedScope() config.ConfigScope {
	return scopeOptions[s.selected].scope
}

func (s *ScopeStep) Init() tea.Cmd {
	return nil
}

func (s *ScopeStep) Update(msg tea.Msg) (Step, tea.Cmd) {
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
			if s.selected < len(scopeOptions)-1 {
				s.selected++
			}
		case "enter":
			return s, func() tea.Msg { return NextStepMsg{} }
		}
	}
	return s, nil
}

func (s *ScopeStep) View() string {
	var b strings.Builder

	b.WriteString(Styles.Title.Render("Configuration Scope"))
	b.WriteString("\n\n")

	b.WriteString("Where should we write the configuration?\n\n")

	for i, opt := range scopeOptions {
		cursor := "  "
		if s.selected == i {
			cursor = Styles.Cursor.Render("► ")
		}

		radio := "○"
		if s.selected == i {
			radio = Styles.Selected.Render("●")
		}

		line := fmt.Sprintf("%s %s %-10s  %s", cursor, radio, opt.name, Styles.Desc.Render(opt.description))
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

func (s *ScopeStep) Name() string {
	return "Scope"
}

func (s *ScopeStep) Info() StepInfo {
	return StepInfo{
		Name: "Scope",
		KeyBindings: []KeyBinding{
			BindingsNavigate,
			BindingsConfirm,
			BindingsBack,
			BindingsQuit,
		},
	}
}
