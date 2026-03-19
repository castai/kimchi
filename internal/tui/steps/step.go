package steps

import "github.com/charmbracelet/bubbletea"

type Step interface {
	Init() tea.Cmd
	Update(tea.Msg) (Step, tea.Cmd)
	View() string
	Name() string
	Info() StepInfo
}

type NextStepMsg struct{}
type PrevStepMsg struct{}
type AbortMsg struct{}
type RemoveStepMsg struct{}
type SkipToolsAndRemoveMsg struct{}
