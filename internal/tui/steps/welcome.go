package steps

import (
	"github.com/charmbracelet/bubbletea"
)

// WelcomeStep displays an initial welcome message and overview
type WelcomeStep struct{}

func NewWelcomeStep() *WelcomeStep {
	return &WelcomeStep{}
}

func (s *WelcomeStep) Init() tea.Cmd {
	return nil
}

func (s *WelcomeStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
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
	return "Start coding with Kimchi's open-source LLMs now!\n\n" +
		"Kimchi gives you instant access to production-ready open-source\n" +
		"LLMs — " + Styles.Selected.Render("GLM 5") + ", " + Styles.Selected.Render("Kimi K2.5") + ", " + Styles.Selected.Render("MiniMax M2.5") + " — via an " + Styles.Selected.Render("OpenAI-compatible") + "\n" +
		"API. No GPUs to provision, no clusters to manage.\n\n" +
		"This wizard will guide you through configuration of your local\n" +
		"coding tools in a few quick steps.\n\n" +
		Styles.Title.Render("Are you ready?") + "\n\n" +
		Styles.Help.Render("Press Enter to begin")
}

func (s *WelcomeStep) Name() string {
	return "Welcome"
}

func (s *WelcomeStep) Info() StepInfo {
	return StepInfo{
		Name: "Welcome to Kimchi by Cast AI",
		KeyBindings: []KeyBinding{
			{Key: "↵", Text: "begin"},
		},
	}
}
