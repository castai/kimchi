package steps

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/update"
)

type cliApplyMsg struct{ err error }
type harnessApplyMsg struct{ err error }

type updateState int

const (
	updateStateChoice          updateState = iota // CLI update choice
	updateStateApplying                           // CLI update in progress
	updateStateCLIDone                            // CLI update finished, transition to harness
	updateStateHarnessPrompt                      // prompt to install harness
	updateStateHarnessApplying                    // harness install/update in progress
	updateStateDone                               // all done
)

func progressText(b update.UpdateStatus) string {
	action := "Updating"
	if !b.Installed() {
		action = "Installing"
	}
	return fmt.Sprintf("%s %s v%s...", action, b.DisplayName, b.LatestVersion)
}

func resultText(b update.UpdateStatus) string {
	action := "Updated"
	if !b.Installed() {
		action = "Installed"
	}
	return fmt.Sprintf("%s %s to v%s", action, b.DisplayName, b.LatestVersion)
}

func errorText(b update.UpdateStatus, r applyResult) string {
	if r.err == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", b.DisplayName, r.err.Error())
}

// applyResult tracks the outcome of an update/install attempt.
type applyResult struct {
	applied bool  // was the update/install successfully applied?
	err     error // error from the apply attempt (nil on success or skip)
}

type UpdateStep struct {
	cli           update.UpdateStatus
	harness       update.UpdateStatus // zero value = no harness to manage
	cliResult     applyResult
	harnessResult applyResult
	state         updateState
	skipSelected  bool
	spinner       spinner.Model
}

func NewUpdateStep(cli, harness update.UpdateStatus) *UpdateStep {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = Styles.Spinner

	initialState := updateStateChoice
	if !cli.HasUpdate && (harness.HasUpdate || harness.Installed()) {
		// CLI is up to date, go directly to harness handling.
		initialState = updateStateHarnessPrompt
	}

	return &UpdateStep{
		cli:     cli,
		harness: harness,
		state:   initialState,
		spinner: s,
	}
}

// hasHarness reports whether there is harness work to consider (update or installed).
func (s *UpdateStep) hasHarness() bool {
	return s.harness.HasUpdate || s.harness.Installed()
}

func (s *UpdateStep) Init() tea.Cmd { return nil }

func (s *UpdateStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case cliApplyMsg:
		s.cliResult.err = msg.err
		s.cliResult.applied = msg.err == nil
		if s.hasHarness() {
			s.state = updateStateCLIDone
			return s, nil
		}
		s.state = updateStateDone
		return s, nil

	case harnessApplyMsg:
		s.harnessResult.err = msg.err
		s.harnessResult.applied = msg.err == nil
		s.state = updateStateDone
		return s, nil

	case spinner.TickMsg:
		if s.state == updateStateApplying || s.state == updateStateHarnessApplying {
			var cmd tea.Cmd
			s.spinner, cmd = s.spinner.Update(msg)
			return s, cmd
		}

	case tea.KeyMsg:
		switch s.state {
		case updateStateChoice:
			return s.handleChoiceKey(msg)
		case updateStateCLIDone:
			return s.handleCLIDoneKey(msg)
		case updateStateHarnessPrompt:
			return s.handleHarnessPromptKey(msg)
		case updateStateDone:
			return s.handleDoneKey(msg)
		}
	}
	return s, nil
}

func (s *UpdateStep) handleChoiceKey(msg tea.KeyMsg) (Step, tea.Cmd) {
	if cmd, ok := s.handleQuit(msg); ok {
		return s, cmd
	}
	switch msg.String() {
	case "up", "k", "down", "j":
		s.skipSelected = !s.skipSelected
	case "enter":
		if s.skipSelected {
			// Skip CLI update.
			if s.hasHarness() {
				s.state = updateStateHarnessPrompt
				s.skipSelected = false
				return s, nil
			}
			return s, func() tea.Msg { return NextStepMsg{} }
		}
		s.state = updateStateApplying
		return s, tea.Batch(s.spinner.Tick, s.applyCLIUpdate())
	}
	return s, nil
}

func (s *UpdateStep) handleCLIDoneKey(msg tea.KeyMsg) (Step, tea.Cmd) {
	if cmd, ok := s.handleQuit(msg); ok {
		return s, cmd
	}
	switch msg.String() {
	case "enter", " ":
		s.transitionToHarness()
		return s, nil
	}
	return s, nil
}

func (s *UpdateStep) handleHarnessPromptKey(msg tea.KeyMsg) (Step, tea.Cmd) {
	if cmd, ok := s.handleQuit(msg); ok {
		return s, cmd
	}
	switch msg.String() {
	case "up", "k", "down", "j":
		s.skipSelected = !s.skipSelected
	case "enter":
		if s.skipSelected {
			// Skip harness.
			s.state = updateStateDone
			return s, nil
		}
		s.state = updateStateHarnessApplying
		return s, tea.Batch(s.spinner.Tick, s.applyHarnessUpdate())
	}
	return s, nil
}

func (s *UpdateStep) handleDoneKey(msg tea.KeyMsg) (Step, tea.Cmd) {
	if cmd, ok := s.handleQuit(msg); ok {
		return s, cmd
	}
	switch msg.String() {
	case "enter", " ":
		if s.cliResult.applied {
			// CLI was updated — exit so the user restarts with the new version.
			return s, func() tea.Msg { return AbortMsg{} }
		}
		return s, func() tea.Msg { return NextStepMsg{} }
	}
	return s, nil
}

func (s *UpdateStep) handleQuit(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "ctrl+c", "q":
		return func() tea.Msg { return AbortMsg{} }, true
	}
	return nil, false
}

func (s *UpdateStep) transitionToHarness() tea.Cmd {
	if !s.hasHarness() || (s.harness.Installed() && !s.harness.HasUpdate) {
		// Nothing to do for harness — skip directly to done.
		s.state = updateStateDone
		return nil
	}
	s.state = updateStateHarnessPrompt
	s.skipSelected = false
	return nil
}

func (s *UpdateStep) applyCLIUpdate() tea.Cmd {
	return func() tea.Msg {
		wf := update.NewCLIWorkflow()
		_, err := wf.Run(context.Background())
		return cliApplyMsg{err: err}
	}
}

func (s *UpdateStep) applyHarnessUpdate() tea.Cmd {
	return func() tea.Msg {
		wf := update.NewHarnessWorkflow()
		_, err := wf.Run(context.Background())
		return harnessApplyMsg{err: err}
	}
}

func (s *UpdateStep) View() string {
	switch s.state {
	case updateStateApplying:
		return s.spinner.View() + " " + progressText(s.cli)

	case updateStateCLIDone:
		view := ""
		if s.cliResult.err != nil {
			view += Styles.Error.Render("CLI update failed: "+s.cliResult.err.Error()) + "\n\n"
		} else {
			view += Styles.Success.Render(resultText(s.cli)) + "\n\n"
		}
		view += Styles.Desc.Render("Press Enter to continue to coding harness...")
		return view

	case updateStateHarnessPrompt:
		return s.viewHarnessPrompt()

	case updateStateHarnessApplying:
		return s.viewCLISummary() +
			s.spinner.View() + " " + progressText(s.harness)

	case updateStateDone:
		return s.viewDone()

	default:
		return s.viewCLIChoice()
	}
}

func (s *UpdateStep) viewCLIChoice() string {
	updateLabel := "  Update now  "
	skipLabel := "  Skip  "
	updateStyle := Styles.Item
	skipStyle := Styles.Item
	if !s.skipSelected {
		updateLabel = "► Update now  "
		updateStyle = Styles.Selected
	} else {
		skipLabel = "► Skip  "
		skipStyle = Styles.Selected
	}

	return Styles.Warning.Render(fmt.Sprintf("Update available: v%s → v%s", s.cli.CurrentVersion, s.cli.LatestVersion)) + "\n\n" +
		updateStyle.Render(updateLabel) + "\n" +
		skipStyle.Render(skipLabel)
}

func (s *UpdateStep) viewHarnessPrompt() string {
	view := s.viewCLISummary()

	label := "Update"
	if !s.harness.Installed() {
		label = "Install"
	}

	installLabel := fmt.Sprintf("  %s %s v%s  ", label, s.harness.DisplayName, s.harness.LatestVersion)
	skipLabel := "  Skip  "
	installStyle := Styles.Item
	skipStyle := Styles.Item
	if !s.skipSelected {
		installLabel = fmt.Sprintf("► %s %s v%s  ", label, s.harness.DisplayName, s.harness.LatestVersion)
		installStyle = Styles.Selected
	} else {
		skipLabel = "► Skip  "
		skipStyle = Styles.Selected
	}

	if s.harness.Installed() {
		view += Styles.Warning.Render(fmt.Sprintf("Coding harness update: v%s → v%s", s.harness.CurrentVersion, s.harness.LatestVersion)) + "\n\n"
	} else {
		view += Styles.Warning.Render("Coding harness is not installed") + "\n\n"
	}
	view += installStyle.Render(installLabel) + "\n" +
		skipStyle.Render(skipLabel)
	return view
}

func (s *UpdateStep) viewCLISummary() string {
	if !s.cli.HasUpdate {
		return ""
	}
	if s.cliResult.applied {
		return Styles.Success.Render(resultText(s.cli)) + "\n\n"
	}
	if s.cliResult.err != nil {
		return Styles.Error.Render("CLI update failed: "+s.cliResult.err.Error()) + "\n\n"
	}
	return Styles.Desc.Render("CLI update skipped") + "\n\n"
}

func (s *UpdateStep) viewDone() string {
	view := ""

	// CLI status.
	if s.cli.HasUpdate {
		if s.cliResult.applied {
			view += Styles.Success.Render(resultText(s.cli)) + "\n"
		} else if s.cliResult.err != nil {
			view += Styles.Error.Render("CLI update failed: "+s.cliResult.err.Error()) + "\n"
		}
	}

	// Harness status.
	if s.hasHarness() {
		if s.harnessResult.err != nil {
			view += Styles.Warning.Render(errorText(s.harness, s.harnessResult)) + "\n"
		} else if s.harnessResult.applied {
			view += Styles.Success.Render(resultText(s.harness)) + "\n"
		}
	}

	view += "\n"
	if s.cliResult.applied {
		view += Styles.Desc.Render("Press Enter to exit. Please re-run kimchi to use the new version.")
	} else if s.cliResult.err != nil || s.harnessResult.err != nil {
		view += Styles.Desc.Render("Press Enter to continue or run `kimchi update` later.")
	} else {
		view += Styles.Desc.Render("Press Enter to continue.")
	}

	return view
}

func (s *UpdateStep) Name() string { return "Update" }

func (s *UpdateStep) Info() StepInfo {
	var bindings []KeyBinding
	switch s.state {
	case updateStateApplying, updateStateHarnessApplying:
		bindings = []KeyBinding{BindingsQuit}
	case updateStateDone, updateStateCLIDone:
		bindings = []KeyBinding{BindingsConfirm}
	default:
		bindings = []KeyBinding{BindingsNavigate, BindingsConfirm, BindingsQuit}
	}
	return StepInfo{Name: "Update", KeyBindings: bindings}
}
