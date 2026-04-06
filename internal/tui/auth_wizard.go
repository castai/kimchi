package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/tui/steps"
)

// authWizard is a minimal bubbletea model that runs a single auth step standalone.
type authWizard struct {
	step    steps.Step
	aborted bool
	done    bool
}

func (w *authWizard) Init() tea.Cmd {
	return tea.Batch(w.step.Init(), tea.EnterAltScreen)
}

func (w *authWizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case steps.NextStepMsg:
		w.done = true
		return w, tea.Quit
	case steps.AbortMsg:
		w.aborted = true
		return w, tea.Quit
	}
	updated, cmd := w.step.Update(msg)
	w.step = updated
	return w, cmd
}

func (w *authWizard) View() string {
	return steps.StepView(w.step.Info(), w.step.View())
}

// RunAuthWizard launches the standalone Cast AI API key auth TUI.
func RunAuthWizard() error {
	w := &authWizard{step: steps.NewAuthStep()}
	p := tea.NewProgram(w, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

