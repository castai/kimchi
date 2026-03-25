package steps

import (
	"context"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/update"
)

type updateApplyMsg struct{ err error }

type updateState int

const (
	updateStateChoice updateState = iota
	updateStateApplying
	updateStateDone
)

type UpdateStep struct {
	currentVersion string
	latestVersion  string
	latestTag      string
	state          updateState
	choice         int
	err            error
	spinner        spinner.Model
}

func NewUpdateStep(currentVersion, latestVersion, latestTag string) *UpdateStep {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = Styles.Spinner
	return &UpdateStep{
		currentVersion: currentVersion,
		latestVersion:  latestVersion,
		latestTag:      latestTag,
		spinner:        s,
	}
}

func (s *UpdateStep) Init() tea.Cmd { return nil }

func (s *UpdateStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case updateApplyMsg:
		s.state = updateStateDone
		s.err = msg.err
		if msg.err == nil {
			return s, nil
		}
		return s, nil

	case spinner.TickMsg:
		if s.state == updateStateApplying {
			var cmd tea.Cmd
			s.spinner, cmd = s.spinner.Update(msg)
			return s, cmd
		}

	case tea.KeyMsg:
		switch s.state {
		case updateStateChoice:
			switch msg.String() {
			case "ctrl+c", "q":
				return s, func() tea.Msg { return AbortMsg{} }
			case "up", "k", "down", "j":
				s.choice = 1 - s.choice
			case "enter":
				if s.choice == 1 {
					return s, func() tea.Msg { return NextStepMsg{} }
				}
				s.state = updateStateApplying
				return s, tea.Batch(s.spinner.Tick, s.applyUpdate())
			}

		case updateStateDone:
			switch msg.String() {
			case "ctrl+c", "q":
				return s, func() tea.Msg { return AbortMsg{} }
			case "enter", " ":
				if s.err == nil {
					return s, func() tea.Msg { return AbortMsg{} }
				}
				return s, func() tea.Msg { return NextStepMsg{} }
			}
		}
	}
	return s, nil
}

func (s *UpdateStep) applyUpdate() tea.Cmd {
	tag := s.latestTag
	return func() tea.Msg {
		execPath, err := update.ResolveExecutablePath()
		if err != nil {
			return updateApplyMsg{err: err}
		}
		err = update.Apply(context.Background(), update.NewGitHubClient(), tag, update.WithExecutablePath(execPath))
		return updateApplyMsg{err: err}
	}
}

func (s *UpdateStep) View() string {
	switch s.state {
	case updateStateApplying:
		return s.spinner.View() + " Updating to v" + s.latestVersion + "..."

	case updateStateDone:
		if s.err != nil {
			return Styles.Error.Render("Update failed: "+s.err.Error()) + "\n\n" +
				Styles.Desc.Render("Press Enter to continue or run `kimchi update` later.")
		}
		return Styles.Success.Render("Updated to v"+s.latestVersion) + "\n\n" +
			Styles.Desc.Render("Press Enter to exit. Please re-run kimchi to use the new version.")

	default:
		updateLabel := "  Update now  "
		skipLabel := "  Skip  "
		updateStyle := Styles.Item
		skipStyle := Styles.Item
		if s.choice == 0 {
			updateLabel = "► Update now  "
			updateStyle = Styles.Selected
		} else {
			skipLabel = "► Skip  "
			skipStyle = Styles.Selected
		}

		return Styles.Warning.Render("Update available: v"+s.currentVersion+" → v"+s.latestVersion) + "\n\n" +
			updateStyle.Render(updateLabel) + "\n" +
			skipStyle.Render(skipLabel)
	}
}

func (s *UpdateStep) Name() string { return "Update" }

func (s *UpdateStep) Info() StepInfo {
	bindings := []KeyBinding{BindingsNavigate, BindingsConfirm, BindingsQuit}
	if s.state == updateStateApplying {
		bindings = []KeyBinding{BindingsQuit}
	} else if s.state == updateStateDone {
		bindings = []KeyBinding{BindingsConfirm}
	}
	return StepInfo{Name: "Update", KeyBindings: bindings}
}
