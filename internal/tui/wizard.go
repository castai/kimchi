package tui

import (
	"context"
	"fmt"
	"slices"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/config"
	"github.com/castai/kimchi/internal/gsd"
	"github.com/castai/kimchi/internal/tools"
	"github.com/castai/kimchi/internal/tui/steps"
	"github.com/castai/kimchi/internal/version"
)

type WizardConfig struct {
	APIKey         string
	Mode           config.ConfigMode
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
	pendingUpdate    *steps.UpdateStep
	pendingGSD       *steps.GSDStep
	pendingTelemetry *steps.TelemetryStep
	pendingConfigure *steps.ConfigureStep
	pendingDone      *steps.DoneStep
	viewport         viewport.Model
	ready            bool
}

func newWizard(ctx context.Context) *wizard {
	welcomeStep := steps.NewWelcomeStep(version.Version)
	authStep := steps.NewAuthStep()
	installStep := steps.NewInstallStep()
	toolsStep := steps.NewToolsStep()
	modeStep := steps.NewModeStep()
	scopeStep := steps.NewScopeStep()

	stepList := []steps.Step{welcomeStep, authStep, installStep, toolsStep, modeStep, scopeStep}

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

		if w.pendingUpdate != nil {
			w.stepList = slices.Insert(w.stepList, w.current+1, steps.Step(w.pendingUpdate))
			w.pendingUpdate = nil
		}

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
	case *steps.WelcomeStep:
		if s.HasUpdate() {
			w.pendingUpdate = steps.NewUpdateStep(version.Version, s.LatestVersion(), s.LatestTag())
		}
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
	case *steps.ModeStep:
		w.config.Mode = s.SelectedMode()
		if w.config.Mode == config.ModeInject {
			// Only skip the scope step if all selected tools are wrappable.
			// IDE tools (Cursor, Zed, etc.) still need override and thus need scope.
			if w.allToolsWrappable() {
				w.config.Scope = config.ScopeGlobal
				w.removePendingStep(func(step steps.Step) bool {
					_, ok := step.(*steps.ScopeStep)
					return ok
				})
				w.scheduleConfigureIfReady()
			}
		}
	case *steps.ScopeStep:
		w.config.Scope = s.SelectedScope()
		w.scheduleConfigureIfReady()
	case *steps.TelemetryStep:
		w.config.TelemetryOptIn = s.OptIn()
		w.scheduleConfigureIfReady()
	case *steps.GSDStep:
		w.config.GSDMigrateFrom = s.GetMigrateInstallations()
		w.config.GSDInstallFor = s.GetInstallTypes()
	case *steps.ConfigureStep:
		w.pendingDone = steps.NewDoneStep(context.Background(), steps.DoneParams{
			APIKey:  w.config.APIKey,
			ToolIDs: w.config.SelectedTools,
		})
	}
}

// scheduleConfigureIfReady creates the GSD and configure steps once all
// tool-specific question steps (telemetry) have been answered.
func (w *wizard) scheduleConfigureIfReady() {
	for _, step := range w.stepList[w.current+1:] {
		if _, ok := step.(*steps.TelemetryStep); ok {
			return // still waiting for telemetry answer
		}
	}
	w.pendingGSD = steps.NewGSDStep(w.config.SelectedTools, w.config.Scope)
	w.pendingConfigure = steps.NewConfigureStep(steps.ConfigureParams{
		ToolIDs:        w.config.SelectedTools,
		Mode:           w.config.Mode,
		Scope:          w.config.Scope,
		TelemetryOptIn: w.config.TelemetryOptIn,
		APIKey:         w.config.APIKey,
	})
}

func (w *wizard) allToolsWrappable() bool {
	for _, id := range w.config.SelectedTools {
		if !id.IsWrappable() {
			return false
		}
	}
	return true
}

func (w *wizard) removePendingStep(match func(steps.Step) bool) {
	for i := w.current + 1; i < len(w.stepList); i++ {
		if match(w.stepList[i]) {
			w.stepList = append(w.stepList[:i], w.stepList[i+1:]...)
			return
		}
	}
}

func (w *wizard) hasClaudeCode() bool {
	return slices.Contains(w.config.SelectedTools, tools.ToolClaudeCode)
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
		var migrateInstallations []gsd.Installation
		for _, install := range cfg.GSDMigrateFrom {
			kimchiPath, err := gsd.KimchiManagedPath(install.Type)
			if err != nil {
				fmt.Printf("Warning: could not determine kimchi path for %s: %v\n", install.Type, err)
				migrateInstallations = append(migrateInstallations, install)
				continue
			}
			if err := gsd.CopyInstallation(install.Path, kimchiPath); err != nil {
				fmt.Printf("Warning: could not copy GSD files to kimchi dir for %s: %v\n", install.Type, err)
				migrateInstallations = append(migrateInstallations, install)
				continue
			}
			agentFiles, err := gsd.ReadAgentFiles(kimchiPath)
			if err != nil {
				fmt.Printf("Warning: could not read GSD agent files for %s: %v\n", install.Type, err)
				migrateInstallations = append(migrateInstallations, install)
				continue
			}
			migrateInstallations = append(migrateInstallations, gsd.Installation{
				Type:       install.Type,
				Path:       kimchiPath,
				AgentFiles: agentFiles,
			})
		}
		if _, err := migrator.Migrate(migrateInstallations); err != nil {
			fmt.Printf("Warning: GSD migration failed: %v\n", err)
		}
	}

	var gsdInstalledTools []string

	if len(cfg.GSDInstallFor) > 0 {
		installer := gsd.NewInstaller()
		for _, installType := range cfg.GSDInstallFor {
			if _, err := installer.Install(installType, string(cfg.Scope)); err != nil {
				fmt.Printf("Warning: GSD installation failed for %s: %v\n", installType, err)
			} else {
				gsdInstalledTools = append(gsdInstalledTools, string(installType))
			}
		}
	}

	for _, install := range cfg.GSDMigrateFrom {
		tool := string(install.Type)
		if !slices.Contains(gsdInstalledTools, tool) {
			gsdInstalledTools = append(gsdInstalledTools, tool)
		}
	}

	if len(gsdInstalledTools) > 0 {
		if err := config.SaveGSDInstalled(gsdInstalledTools); err != nil {
			fmt.Printf("Warning: could not save GSD installation state: %v\n", err)
		}
	}

	return cfg, nil
}
