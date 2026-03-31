package steps

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/update"
)

type updateCheckMsg struct {
	latestVersion string
	latestTag     string
	hasUpdate     bool
}

// WelcomeStep displays an initial welcome message and checks for updates in the background.
type WelcomeStep struct {
	currentVersion string
	latestVersion  string
	latestTag      string
	hasUpdate      bool
}

func NewWelcomeStep(currentVersion string) *WelcomeStep {
	return &WelcomeStep{
		currentVersion: currentVersion,
	}
}

func (s *WelcomeStep) Init() tea.Cmd {
	if update.IsUpdateCheckDisabled() {
		return nil
	}
	currentVersion := s.currentVersion
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		res, err := update.Check(ctx, update.NewGitHubClient(), currentVersion)
		if err != nil {
			return updateCheckMsg{}
		}
		hasUpdate := res.LatestVersion.GreaterThan(&res.CurrentVersion)
		return updateCheckMsg{
			latestVersion: res.LatestVersion.String(),
			latestTag:     res.LatestTag,
			hasUpdate:     hasUpdate,
		}
	}
}

func (s *WelcomeStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case updateCheckMsg:
		if msg.hasUpdate {
			s.hasUpdate = true
			s.latestVersion = msg.latestVersion
			s.latestTag = msg.latestTag
		}
		return s, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return s, func() tea.Msg { return AbortMsg{} }
		case "enter", " ":
			return s, func() tea.Msg { return NextStepMsg{} }
		}
	}
	return s, nil
}

func (s *WelcomeStep) View() string {
	view := "Start coding with open-source LLMs now!\n\n" +
		"Kimchi gives you instant access to production-ready open-source\n" +
		"LLMs — " + Styles.Selected.Render("Kimi K2.5") + ", " + Styles.Selected.Render("GLM 5") + ", " + Styles.Selected.Render("MiniMax M2.5") + " — via an " + Styles.Selected.Render("OpenAI-compatible") + "\n" +
		"API. No GPUs to provision, no clusters to manage.\n\n" +
		"This wizard will guide you through configuration of your local\n" +
		"coding tools in a few quick steps.\n\n"

	view += Styles.Title.Render("Are you ready?") + "\n\n" +
		Styles.Help.Render("Press Enter to begin")
	return view
}

func (s *WelcomeStep) HasUpdate() bool       { return s.hasUpdate }
func (s *WelcomeStep) LatestVersion() string { return s.latestVersion }
func (s *WelcomeStep) LatestTag() string     { return s.latestTag }

func (s *WelcomeStep) Name() string {
	return "Welcome"
}

func (s *WelcomeStep) Info() StepInfo {
	return StepInfo{
		Name:        "Welcome to Kimchi by Cast AI",
		KeyBindings: []KeyBinding{{Key: "↵", Text: "begin"}},
	}
}
