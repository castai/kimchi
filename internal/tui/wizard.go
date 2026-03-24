package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/config"
	"github.com/castai/kimchi/internal/gsd"
	"github.com/castai/kimchi/internal/tools"
	"github.com/castai/kimchi/internal/tui/steps"
)

type WizardConfig struct {
	APIKey         string
	SelectedTools  []tools.ToolID
	Scope          config.ConfigScope
	TelemetryOptIn bool
	GSDMigrateFrom []gsd.Installation
	GSDInstallFor  []gsd.InstallationType
}

type wizard struct {
	stepList         []steps.Step
	current          int
	config           WizardConfig
	finished         bool
	aborted          bool
	pendingGSD       *steps.GSDStep
	pendingTelemetry *steps.TelemetryStep
	pendingConfigure *steps.ConfigureStep
	pendingDone      *steps.DoneStep
	viewport         viewport.Model
	ready            bool
}

func newWizard(ctx context.Context) *wizard {
	welcomeStep := steps.NewWelcomeStep()
	authStep := steps.NewAuthStep()
	installStep := steps.NewInstallStep()
	toolsStep := steps.NewToolsStep()
	scopeStep := steps.NewScopeStep()

	stepList := []steps.Step{welcomeStep, authStep, installStep, toolsStep, scopeStep}

	return &wizard{
		stepList: stepList,
		current:  0,
	}
}

func (w *wizard) Init() tea.Cmd {
	if len(w.stepList) == 0 {
		return tea.Quit
	}
	return tea.Batch(w.stepList[0].Init(), tea.EnterAltScreen)
}

func (w *wizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w.viewport = viewport.New(msg.Width, msg.Height)
		w.viewport.Width = msg.Width
		w.viewport.Height = msg.Height
		w.ready = true
		return w, nil

	case steps.NextStepMsg:
		w.collectStepResult()

		if w.pendingTelemetry != nil {
			w.stepList = append(w.stepList, w.pendingTelemetry)
			w.pendingTelemetry = nil
		}

		if w.pendingGSD != nil {
			w.stepList = append(w.stepList, w.pendingGSD)
			w.pendingGSD = nil
		}

		if w.pendingConfigure != nil {
			w.stepList = append(w.stepList, w.pendingConfigure)
			w.pendingConfigure = nil
		}

		if w.pendingDone != nil {
			w.stepList = append(w.stepList, w.pendingDone)
			w.pendingDone = nil
		}

		if w.current >= len(w.stepList)-1 {
			w.finished = true
			return w, tea.Quit
		}
		w.current++

		if w.current < len(w.stepList) {
			return w, w.stepList[w.current].Init()
		}
		return w, tea.Quit

	case steps.PrevStepMsg:
		if w.current > 0 {
			w.current--
			if w.current >= 0 {
				return w, w.stepList[w.current].Init()
			}
		}
		return w, nil

	case steps.RemoveStepMsg:
		w.collectStepResult()
		w.stepList = append(w.stepList[:w.current], w.stepList[w.current+1:]...)
		if w.current < len(w.stepList) {
			return w, w.stepList[w.current].Init()
		}
		return w, tea.Quit

	case steps.SkipToolsAndRemoveMsg:
		w.collectStepResult()
		w.stepList = append(w.stepList[:w.current], w.stepList[w.current+1:]...)
		for i, step := range w.stepList {
			if _, ok := step.(*steps.ToolsStep); ok {
				w.stepList = append(w.stepList[:i], w.stepList[i+1:]...)
				break
			}
		}
		if w.current < len(w.stepList) {
			return w, w.stepList[w.current].Init()
		}
		return w, tea.Quit

	case steps.AbortMsg:
		w.aborted = true
		return w, tea.Quit
	}

	var cmd tea.Cmd
	if w.ready {
		w.viewport, cmd = w.viewport.Update(msg)
	}

	if w.current < len(w.stepList) {
		updatedStep, stepCmd := w.stepList[w.current].Update(msg)
		w.stepList[w.current] = updatedStep
		if cmd != nil {
			return w, tea.Batch(cmd, stepCmd)
		}
		return w, stepCmd
	}

	return w, cmd
}

func (w *wizard) View() string {
	if w.current >= len(w.stepList) {
		return ""
	}

	step := w.stepList[w.current]

	info := step.Info()
	content := step.View()

	fullView := steps.StepView(info, content)

	if w.ready {
		w.viewport.SetContent(fullView)
		return w.viewport.View()
	}

	return fullView
}

func (w *wizard) collectStepResult() {
	if w.current >= len(w.stepList) {
		return
	}
	step := w.stepList[w.current]
	switch s := step.(type) {
	case *steps.AuthStep:
		w.config.APIKey = s.APIKey()
	case *steps.InstallStep:
		if s.HasInstalledTools() {
			w.config.SelectedTools = s.AutoSelectedTools()
		} else if s.ShouldSkipToolsStep() {
			w.config.SelectedTools = []tools.ToolID{s.SelectedTool()}
		}
	case *steps.ToolsStep:
		w.config.SelectedTools = s.SelectedTools()
		if w.hasClaudeCode() {
			w.pendingTelemetry = steps.NewTelemetryStep()
		}
	case *steps.ScopeStep:
		w.config.Scope = s.SelectedScope()
		if !w.hasClaudeCode() {
			w.pendingGSD = steps.NewGSDStep(w.config.SelectedTools, w.config.Scope)
			w.pendingConfigure = steps.NewConfigureStep(w.config.SelectedTools, w.config.Scope, w.config.TelemetryOptIn)
		}
	case *steps.TelemetryStep:
		w.config.TelemetryOptIn = s.OptIn()
		w.pendingGSD = steps.NewGSDStep(w.config.SelectedTools, w.config.Scope)
		w.pendingConfigure = steps.NewConfigureStep(w.config.SelectedTools, w.config.Scope, w.config.TelemetryOptIn)
	case *steps.GSDStep:
		w.config.GSDMigrateFrom = s.GetMigrateInstallations()
		w.config.GSDInstallFor = s.GetInstallTypes()
	case *steps.ConfigureStep:
		w.pendingDone = steps.NewDoneStep(context.Background(), w.config.APIKey, w.config.SelectedTools)
	}
}

func (w *wizard) hasClaudeCode() bool {
	for _, toolID := range w.config.SelectedTools {
		if toolID == tools.ToolClaudeCode {
			return true
		}
	}
	return false
}

func RunWizard() (*WizardConfig, error) {
	ctx := context.Background()
	w := newWizard(ctx)

	p := tea.NewProgram(w, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("run wizard: %w", err)
	}

	finalWizard, ok := finalModel.(*wizard)
	if !ok {
		return nil, fmt.Errorf("unexpected model type")
	}

	if finalWizard.aborted {
		return nil, nil
	}

	cfg := &finalWizard.config

	if len(cfg.GSDMigrateFrom) > 0 {
		migrator := gsd.NewMigrator()
		if _, err := migrator.Migrate(cfg.GSDMigrateFrom); err != nil {
			fmt.Printf("Warning: GSD migration failed: %v\n", err)
		}
	}

	if len(cfg.GSDInstallFor) > 0 {
		installer := gsd.NewInstaller()
		for _, installType := range cfg.GSDInstallFor {
			if _, err := installer.Install(installType, string(cfg.Scope)); err != nil {
				fmt.Printf("Warning: GSD installation failed for %s: %v\n", installType, err)
			}
		}
	}

	return cfg, nil
}
