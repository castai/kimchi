package steps

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type installItemStatus int

const (
	installItemPending installItemStatus = iota
	installItemWriting
	installItemDone
	installItemFailed
)

type installProgressItem struct {
	label  string
	status installItemStatus
	err    error
}

type installStartMsg struct{}
type installWriteCompleteMsg struct{ err error }

// InstallProgressStep performs the actual install (via writeFn) and shows the result.
type InstallProgressStep struct {
	items     []installProgressItem
	writeFn   func() error
	state     installProgressState
	err       error
	spin      spinner.Model
	startOnce sync.Once
}

type installProgressState int

const (
	installProgressWaiting installProgressState = iota
	installProgressRunning
	installProgressDone
	installProgressError
)

// NewInstallProgressStep creates the final install step.
// itemLabels is a list of human-readable descriptions of what will be written,
// shown as a checklist. writeFn performs the actual work.
func NewInstallProgressStep(itemLabels []string, writeFn func() error) *InstallProgressStep {
	items := make([]installProgressItem, len(itemLabels))
	for i, l := range itemLabels {
		items[i] = installProgressItem{label: l, status: installItemPending}
	}
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return &InstallProgressStep{
		items:   items,
		writeFn: writeFn,
		spin:    sp,
	}
}

// SetWriteFn updates the write function after the step is created.
// Called by the wizard once all decisions have been collected.
func (s *InstallProgressStep) SetWriteFn(fn func() error) {
	s.writeFn = fn
}

func (s *InstallProgressStep) Init() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return installStartMsg{}
	})
}

func (s *InstallProgressStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case installStartMsg:
		var cmd tea.Cmd
		s.startOnce.Do(func() {
			s.state = installProgressRunning
			for i := range s.items {
				s.items[i].status = installItemWriting
			}
			cmd = tea.Batch(s.spin.Tick, s.doInstall())
		})
		return s, cmd

	case installWriteCompleteMsg:
		if msg.err != nil {
			s.state = installProgressError
			s.err = msg.err
			for i := range s.items {
				if s.items[i].status == installItemWriting {
					s.items[i].status = installItemFailed
				}
			}
		} else {
			s.state = installProgressDone
			for i := range s.items {
				s.items[i].status = installItemDone
			}
		}
		return s, nil

	case spinner.TickMsg:
		if s.state == installProgressRunning {
			var cmd tea.Cmd
			s.spin, cmd = s.spin.Update(msg)
			return s, cmd
		}

	case tea.KeyMsg:
		if s.state == installProgressDone || s.state == installProgressError {
			switch msg.String() {
			case "enter", "ctrl+c", "q":
				return s, func() tea.Msg { return NextStepMsg{} }
			}
		}
	}
	return s, nil
}

func (s *InstallProgressStep) doInstall() tea.Cmd {
	return func() tea.Msg {
		return installWriteCompleteMsg{err: s.writeFn()}
	}
}

func (s *InstallProgressStep) View() string {
	var b strings.Builder

	spinChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinIdx := 0

	for _, item := range s.items {
		var icon, label string
		switch item.status {
		case installItemDone:
			icon = Styles.Success.Render("✓")
			label = Styles.Success.Render(" " + item.label)
		case installItemFailed:
			icon = Styles.Error.Render("✗")
			label = Styles.Error.Render(" " + item.label)
		case installItemWriting:
			spin := spinChars[spinIdx%len(spinChars)]
			spinIdx++
			icon = Styles.Spinner.Render(spin)
			label = Styles.Spinner.Render(" " + item.label)
		default:
			icon = "○"
			label = " " + item.label
		}
		b.WriteString(fmt.Sprintf("  %s%s\n", icon, label))
	}

	if s.state == installProgressDone {
		b.WriteString("\n")
		b.WriteString(Styles.Success.Render("✓ Recipe installed successfully!"))
		b.WriteString("\n\n")
		b.WriteString(Styles.Help.Render("Press enter to exit"))
	} else if s.state == installProgressError {
		b.WriteString("\n")
		b.WriteString(Styles.Error.Render(fmt.Sprintf("✗ Install failed: %v", s.err)))
		b.WriteString("\n\n")
		b.WriteString(Styles.Help.Render("Press enter to exit"))
	}

	return b.String()
}

func (s *InstallProgressStep) Name() string { return "Installing" }

func (s *InstallProgressStep) Info() StepInfo {
	bindings := []KeyBinding{}
	if s.state == installProgressDone || s.state == installProgressError {
		bindings = []KeyBinding{BindingsConfirm}
	}
	return StepInfo{
		Name:        "Installing",
		KeyBindings: bindings,
	}
}
