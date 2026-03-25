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

type welcomeState int

const (
	welcomeStateWelcome welcomeState = iota
	welcomeStateChoice
	welcomeStateApplying
	welcomeStateApplied
	welcomeStateError
)

// WelcomeStep displays an initial welcome message and checks for updates in the background.
type WelcomeStep struct {
	currentVersion string
	latestVersion  string
	releaseURL     string
	hasUpdate      bool
	checkDone      bool
	state          welcomeState
	applyErr       error
	choice         int
	spinner        spinner.Model
}

func NewWelcomeStep(currentVersion string) *WelcomeStep {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = Styles.Spinner
	return &WelcomeStep{
		currentVersion: currentVersion,
		spinner:        s,
	}
}

func (s *WelcomeStep) Init() tea.Cmd {
	return s.checkForUpdate()
}

func (s *WelcomeStep) checkForUpdate() tea.Cmd {
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

func (s *WelcomeStep) applyUpdate() tea.Cmd {
	latest := s.latestVersion
	return func() tea.Msg {
		client := update.NewGitHubClient()
		err := update.Apply(context.Background(), client, "v"+latest)
		return updateApplyMsg{err: err}
	}
}

func (s *WelcomeStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case updateCheckMsg:
		s.checkDone = true
		if msg.err == nil && msg.hasUpdate {
			s.hasUpdate = true
			s.latestVersion = msg.latestVersion
			s.releaseURL = msg.releaseURL
		}
		return s, nil

	case updateApplyMsg:
		if msg.err != nil {
			s.state = welcomeStateError
			s.applyErr = msg.err
			return s, nil
		}
		s.state = welcomeStateApplied
		return s, nil

	case spinner.TickMsg:
		if s.state == welcomeStateApplying {
			var cmd tea.Cmd
			s.spinner, cmd = s.spinner.Update(msg)
			return s, cmd
		}

	case tea.KeyMsg:
		switch s.state {
		case welcomeStateWelcome:
			switch msg.String() {
			case "ctrl+c", "q":
				return s, func() tea.Msg { return AbortMsg{} }
			case "enter", " ":
				if s.checkDone && s.hasUpdate {
					s.state = welcomeStateChoice
					return s, nil
				}
				return s, func() tea.Msg { return NextStepMsg{} }
			}

		case welcomeStateChoice:
			switch msg.String() {
			case "ctrl+c", "q":
				return s, func() tea.Msg { return AbortMsg{} }
			case "esc":
				return s, func() tea.Msg { return NextStepMsg{} }
			case "up", "k", "down", "j":
				if s.choice == 0 {
					s.choice = 1
				} else {
					s.choice = 0
				}
			case "enter":
				if s.choice == 1 {
					return s, func() tea.Msg { return NextStepMsg{} }
				}
				s.state = welcomeStateApplying
				return s, tea.Batch(s.spinner.Tick, s.applyUpdate())
			}

		case welcomeStateApplied:
			switch msg.String() {
			case "ctrl+c", "q", "enter", "esc":
				return s, func() tea.Msg { return AbortMsg{} }
			}

		case welcomeStateError:
			switch msg.String() {
			case "ctrl+c", "q":
				return s, func() tea.Msg { return AbortMsg{} }
			case "enter", "esc":
				return s, func() tea.Msg { return NextStepMsg{} }
			}
		}
	}
	return s, nil
}

func (s *WelcomeStep) View() string {
	welcome := "Start coding with open-source LLMs now!\n\n" +
		"Kimchi gives you instant access to production-ready open-source\n" +
		"LLMs — " + Styles.Selected.Render("GLM 5") + ", " + Styles.Selected.Render("Kimi K2.5") + ", " + Styles.Selected.Render("MiniMax M2.5") + " — via an " + Styles.Selected.Render("OpenAI-compatible") + "\n" +
		"API. No GPUs to provision, no clusters to manage.\n\n" +
		"This wizard will guide you through configuration of your local\n" +
		"coding tools in a few quick steps.\n\n"

	switch s.state {
	case welcomeStateChoice:
		var b strings.Builder
		b.WriteString(welcome)
		b.WriteString(Styles.Warning.Render("Update available: v"+s.currentVersion+" → v"+s.latestVersion))
		b.WriteString("\n")
		if s.releaseURL != "" {
			b.WriteString(Styles.Desc.Render("Release notes: " + s.releaseURL))
			b.WriteString("\n")
		}
		b.WriteString("\n")

		updateStyle := Styles.Item
		skipStyle := Styles.Item
		if s.choice == 0 {
			updateStyle = Styles.Selected
		} else {
			skipStyle = Styles.Selected
		}
		updateText := "  Update now  "
		skipText := "  Continue without updating  "
		if s.choice == 0 {
			updateText = "► Update now  "
		} else {
			skipText = "► Continue without updating  "
		}
		b.WriteString(updateStyle.Render(updateText))
		b.WriteString("\n")
		b.WriteString(skipStyle.Render(skipText))
		return b.String()

	case welcomeStateApplying:
		return welcome +
			s.spinner.View() + " Updating to v" + s.latestVersion + "..."

	case welcomeStateApplied:
		return welcome +
			Styles.Success.Render("✓") + " Updated to v" + s.latestVersion + ". Please restart kimchi."

	case welcomeStateError:
		return welcome +
			Styles.Error.Render("✗") + " Update failed: " + s.applyErr.Error() + "\n\n" +
			Styles.Desc.Render("Press Enter to continue or run `kimchi update` later.")

	default:
		return welcome +
			Styles.Title.Render("Are you ready?") + "\n\n" +
			Styles.Help.Render("Press Enter to begin")
	}
}

func (s *WelcomeStep) Name() string {
	return "Welcome"
}

func (s *WelcomeStep) Info() StepInfo {
	var bindings []KeyBinding
	switch s.state {
	case welcomeStateApplying:
		bindings = []KeyBinding{BindingsQuit}
	case welcomeStateChoice:
		bindings = []KeyBinding{BindingsNavigate, BindingsConfirm, BindingsQuit}
	case welcomeStateApplied, welcomeStateError:
		bindings = []KeyBinding{BindingsConfirm}
	default:
		bindings = []KeyBinding{{Key: "↵", Text: "begin"}}
	}
	return StepInfo{Name: "Welcome to Kimchi by Cast AI", KeyBindings: bindings}
}
