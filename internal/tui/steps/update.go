package steps

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/update"
)

type updateCheckMsg struct {
	latestVersion string
	releaseURL    string
	hasUpdate     bool
	err           error
}

type updateApplyMsg struct {
	err error
}

type updateAppliedMsg struct{}

type UpdateStep struct {
	choice         int
	currentVersion string
	latestVersion  string
	releaseURL     string
	hasUpdate      bool
	checking       bool
	applying       bool
	applied        bool
	applyErr       error
	spinner        spinner.Model
}

func NewUpdateStep(currentVersion string) *UpdateStep {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = Styles.Spinner
	return &UpdateStep{
		currentVersion: currentVersion,
		checking:       true,
		spinner:        s,
	}
}

func (s *UpdateStep) Init() tea.Cmd {
	return tea.Batch(s.spinner.Tick, s.checkForUpdate())
}

func (s *UpdateStep) checkForUpdate() tea.Cmd {
	currentVersion := s.currentVersion
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		res, err := update.Check(ctx, update.NewGitHubClient(), currentVersion)
		if err != nil {
			return updateCheckMsg{err: err}
		}
		hasUpdate := res.LatestVersion.GreaterThan(&res.CurrentVersion)
		return updateCheckMsg{
			latestVersion: res.LatestVersion.String(),
			releaseURL:    res.ReleaseURL,
			hasUpdate:     hasUpdate,
		}
	}
}

func (s *UpdateStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case updateCheckMsg:
		s.checking = false
		if msg.err != nil || !msg.hasUpdate {
			return s, func() tea.Msg { return NextStepMsg{} }
		}
		s.hasUpdate = true
		s.latestVersion = msg.latestVersion
		s.releaseURL = msg.releaseURL
		return s, nil

	case updateApplyMsg:
		s.applying = false
		if msg.err != nil {
			s.applyErr = msg.err
			return s, nil
		}
		s.applied = true
		return s, tea.Tick(time.Second, func(time.Time) tea.Msg {
			return updateAppliedMsg{}
		})

	case updateAppliedMsg:
		return s, func() tea.Msg { return NextStepMsg{} }

	case spinner.TickMsg:
		var cmd tea.Cmd
		s.spinner, cmd = s.spinner.Update(msg)
		return s, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return s, func() tea.Msg { return AbortMsg{} }
		case "esc":
			if !s.applying {
				return s, func() tea.Msg { return NextStepMsg{} }
			}
		case "up", "k", "down", "j":
			if s.hasUpdate && !s.applying && !s.applied && s.applyErr == nil {
				if s.choice == 0 {
					s.choice = 1
				} else {
					s.choice = 0
				}
			}
		case "enter":
			if s.applyErr != nil {
				return s, func() tea.Msg { return NextStepMsg{} }
			}
			if s.applied {
				return s, func() tea.Msg { return NextStepMsg{} }
			}
			if s.hasUpdate && !s.applying {
				if s.choice == 1 {
					return s, func() tea.Msg { return NextStepMsg{} }
				}
				s.applying = true
				return s, tea.Batch(s.spinner.Tick, s.applyUpdate())
			}
		}
	}
	return s, nil
}

func (s *UpdateStep) applyUpdate() tea.Cmd {
	latest := s.latestVersion
	return func() tea.Msg {
		client := update.NewGitHubClient()
		err := update.Apply(context.Background(), client, "v"+latest)
		return updateApplyMsg{err: err}
	}
}

func (s *UpdateStep) View() string {
	var b strings.Builder

	b.WriteString(Styles.Title.Render("Update"))
	b.WriteString("\n\n")

	if s.checking {
		b.WriteString(s.spinner.View())
		b.WriteString(" Checking for updates...")
		return b.String()
	}

	if s.applying {
		b.WriteString(s.spinner.View())
		b.WriteString(" Updating to v" + s.latestVersion + "...")
		return b.String()
	}

	if s.applied {
		b.WriteString(Styles.Success.Render("✓"))
		b.WriteString(" Updated to v" + s.latestVersion)
		return b.String()
	}

	if s.applyErr != nil {
		b.WriteString(Styles.Error.Render("✗"))
		b.WriteString(" Update failed: " + s.applyErr.Error())
		b.WriteString("\n\n")
		b.WriteString(Styles.Desc.Render("Press Enter to continue or run `kimchi update` later."))
		return b.String()
	}

	b.WriteString(Styles.Warning.Render("Update available: v"+s.currentVersion+" → v"+s.latestVersion))
	b.WriteString("\n")
	if s.releaseURL != "" {
		b.WriteString(Styles.Desc.Render("Release notes: " + s.releaseURL))
	}
	b.WriteString("\n\n")

	updateStyle := Styles.Item
	skipStyle := Styles.Item

	if s.choice == 0 {
		updateStyle = Styles.Selected
	} else {
		skipStyle = Styles.Selected
	}

	updateText := "  Update now  "
	skipText := "  Skip  "

	if s.choice == 0 {
		updateText = "► Update now  "
	} else {
		skipText = "► Skip  "
	}

	b.WriteString(updateStyle.Render(updateText))
	b.WriteString("\n")
	b.WriteString(skipStyle.Render(skipText))

	return b.String()
}

func (s *UpdateStep) Name() string {
	return "Update"
}

func (s *UpdateStep) Info() StepInfo {
	if s.checking || s.applying {
		return StepInfo{
			Name: "Update",
			KeyBindings: []KeyBinding{
				BindingsQuit,
			},
		}
	}
	if s.applied || s.applyErr != nil {
		return StepInfo{
			Name: "Update",
			KeyBindings: []KeyBinding{
				BindingsConfirm,
			},
		}
	}
	return StepInfo{
		Name: "Update",
		KeyBindings: []KeyBinding{
			BindingsNavigate,
			BindingsConfirm,
			BindingsQuit,
		},
	}
}
