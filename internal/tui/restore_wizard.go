package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/tui/steps"
)

type restoreWizard struct {
	stepList    []steps.Step
	current     int
	pickerStep  *steps.RestorePickerStep
	confirmStep *steps.RestoreConfirmStep
}

func newRestoreWizard() *restoreWizard {
	picker := steps.NewRestorePickerStep()
	return &restoreWizard{
		stepList:   []steps.Step{picker},
		pickerStep: picker,
	}
}

func (w *restoreWizard) Init() tea.Cmd {
	return tea.Batch(w.stepList[0].Init(), tea.EnterAltScreen)
}

func (w *restoreWizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case steps.NextStepMsg:
		// After picker: inject confirm step with selected slot.
		if picker, ok := w.stepList[w.current].(*steps.RestorePickerStep); ok {
			if slot := picker.SelectedSlot(); slot != nil && w.confirmStep == nil {
				confirm := steps.NewRestoreConfirmStep(slot)
				w.confirmStep = confirm
				w.stepList = append(w.stepList, confirm)
			}
		}
		if w.current >= len(w.stepList)-1 {
			return w, tea.Quit
		}
		w.current++
		return w, w.stepList[w.current].Init()

	case steps.PrevStepMsg:
		if w.current > 0 {
			// Going back from confirm: allow re-picking.
			if w.confirmStep != nil && w.current == len(w.stepList)-1 {
				w.stepList = w.stepList[:len(w.stepList)-1]
				w.confirmStep = nil
			}
			w.current--
			return w, w.stepList[w.current].Init()
		}
		return w, nil

	case steps.AbortMsg:
		return w, tea.Quit
	}

	updatedStep, cmd := w.stepList[w.current].Update(msg)
	w.stepList[w.current] = updatedStep
	return w, cmd
}

func (w *restoreWizard) View() string {
	if w.current >= len(w.stepList) {
		return ""
	}
	step := w.stepList[w.current]
	return steps.StepView(step.Info(), step.View())
}

// RunRestoreWizard launches the backup restore TUI.
func RunRestoreWizard() error {
	w := newRestoreWizard()
	p := tea.NewProgram(w, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
