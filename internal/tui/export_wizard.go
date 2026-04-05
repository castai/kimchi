package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/config"
	"github.com/castai/kimchi/internal/recipe"
	"github.com/castai/kimchi/internal/tools"
	"github.com/castai/kimchi/internal/tui/steps"
)

const defaultOutputPath = "kimchi-recipe.yaml"

// ExportWizardOptions are the options for the export wizard, set from CLI flags.
type ExportWizardOptions struct {
	OutputPath string
	Name       string   // pre-fills the recipe name prompt
	Tags       []string // pre-fills tags; combined with any entered in the wizard
	DryRun     bool     // print to stdout instead of writing a file
}

// exportWizard is a standalone bubbletea model for the recipe export flow.
type exportWizard struct {
	stepList     []steps.Step
	current      int
	opts         recipe.ExportOptions
	finished     bool
	aborted      bool
	outputPath   string
	dryRun       bool
	selectedTool tools.ToolID
	scope        config.ConfigScope

	// typed references for result collection
	toolStep    *steps.ExportToolStep
	scopeStep   *steps.ExportScopeStep
	metaStep    *steps.ExportMetaStep
	useCaseStep *steps.ExportUseCaseStep
	assetsStep  *steps.ExportAssetsStep
	outputStep  *steps.ExportOutputStep // nil when --output flag was provided
	confirmStep *steps.ExportConfirmStep
}

// newExportWizard builds the wizard from the given options.
func newExportWizard(wizOpts ExportWizardOptions) *exportWizard {
	toolStep := steps.NewExportToolStep()
	scopeStep := steps.NewExportScopeStep()
	meta := steps.NewExportMetaStep(wizOpts.Name)
	useCase := steps.NewExportUseCaseStep()

	w := &exportWizard{
		outputPath:  wizOpts.OutputPath,
		dryRun:      wizOpts.DryRun,
		scope:       config.ScopeGlobal, // default; updated when scope step completes
		toolStep:    toolStep,
		scopeStep:   scopeStep,
		metaStep:    meta,
		useCaseStep: useCase,
		opts: recipe.ExportOptions{
			Tags: wizOpts.Tags,
		},
	}

	// assetsStep is created lazily in collectStepResult once scope is known.

	// writeFn is called by the confirm step when the user presses Enter.
	writeFn := func() ([]string, error) {
		var (
			a   *recipe.OpenCodeAssets
			err error
		)
		switch w.selectedTool {
		case tools.ToolOpenCode:
			switch w.scope {
			case config.ScopeProject:
				a, err = recipe.ReadProjectOpenCodeAssets()
			default:
				a, err = recipe.ReadGlobalOpenCodeAssets()
			}
			if err != nil {
				return nil, fmt.Errorf("read opencode assets: %w", err)
			}
			r, err := recipe.Build(a, w.opts)
			if err != nil {
				return nil, fmt.Errorf("build recipe: %w", err)
			}
			if w.dryRun {
				return a.UnresolvedRefs, recipe.WriteYAMLTo(os.Stdout, r)
			}
			return a.UnresolvedRefs, recipe.WriteYAML(w.outputPath, r)
		default:
			return nil, fmt.Errorf("unsupported tool: %s", w.selectedTool)
		}
	}

	// Placeholder path for the confirm step — updated later in collectStepResult.
	confirmOutputPath := wizOpts.OutputPath
	if confirmOutputPath == "" {
		confirmOutputPath = defaultOutputPath
	}
	if wizOpts.DryRun {
		confirmOutputPath = "<stdout>"
	}
	confirm := steps.NewExportConfirmStep(confirmOutputPath, writeFn, "", "", "", nil)
	w.confirmStep = confirm

	// Assets step placeholder — replaced with a scope-aware instance after the
	// scope step completes. Use global scope as the initial default.
	assetsStep := steps.NewExportAssetsStep(config.ScopeGlobal)
	w.assetsStep = assetsStep

	stepList := []steps.Step{toolStep, scopeStep, meta, useCase, assetsStep}
	if wizOpts.OutputPath == "" && !wizOpts.DryRun {
		outputStep := steps.NewExportOutputStep(defaultOutputPath)
		w.outputStep = outputStep
		w.outputPath = defaultOutputPath
		stepList = append(stepList, outputStep)
	}
	stepList = append(stepList, confirm)
	w.stepList = stepList
	return w
}

func (w *exportWizard) Init() tea.Cmd {
	if len(w.stepList) == 0 {
		return tea.Quit
	}
	return tea.Batch(w.stepList[0].Init(), tea.EnterAltScreen)
}

func (w *exportWizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case steps.NextStepMsg:
		w.collectStepResult()
		if w.current >= len(w.stepList)-1 {
			w.finished = true
			return w, tea.Quit
		}
		w.current++
		return w, w.stepList[w.current].Init()

	case steps.PrevStepMsg:
		if w.current > 0 {
			w.current--
			return w, w.stepList[w.current].Init()
		}
		return w, nil

	case steps.AbortMsg:
		w.aborted = true
		return w, tea.Quit
	}

	updatedStep, cmd := w.stepList[w.current].Update(msg)
	w.stepList[w.current] = updatedStep
	return w, cmd
}

func (w *exportWizard) View() string {
	if w.current >= len(w.stepList) {
		return ""
	}
	step := w.stepList[w.current]
	return steps.StepView(step.Info(), step.View())
}

func (w *exportWizard) collectStepResult() {
	if w.current >= len(w.stepList) {
		return
	}
	switch s := w.stepList[w.current].(type) {
	case *steps.ExportToolStep:
		w.selectedTool = s.SelectedTool()

	case *steps.ExportScopeStep:
		w.scope = s.SelectedScope()
		// Replace the assets step with a scope-aware instance and update stepList.
		assetsStep := steps.NewExportAssetsStep(w.scope)
		w.assetsStep = assetsStep
		for i, step := range w.stepList {
			if _, ok := step.(*steps.ExportAssetsStep); ok {
				w.stepList[i] = assetsStep
				break
			}
		}

	case *steps.ExportMetaStep:
		w.opts.Name = s.RecipeName()
		w.opts.Author = s.Author()
		w.opts.Description = s.Description()

	case *steps.ExportUseCaseStep:
		w.opts.UseCase = s.SelectedUseCase()

	case *steps.ExportAssetsStep:
		w.opts.IncludeAgentsMD = s.IncludeAgentsMD()
		w.opts.IncludeSkills = s.IncludeSkills()
		w.opts.IncludeCustomCommands = s.IncludeCustomCommands()
		w.opts.IncludeAgents = s.IncludeAgents()
		w.opts.IncludeTUI = s.IncludeTUI()
		w.opts.IncludeThemeFiles = s.IncludeThemeFiles()
		w.opts.IncludePluginFiles = s.IncludePluginFiles()
		w.opts.IncludeToolFiles = s.IncludeToolFiles()
		if w.outputStep == nil {
			w.confirmStep.SetSummary(w.opts.Name, w.opts.Author, w.opts.UseCase, w.includedLabels())
		}

	case *steps.ExportOutputStep:
		w.outputPath = s.OutputPath()
		w.confirmStep.SetOutputPath(w.outputPath)
		w.confirmStep.SetSummary(w.opts.Name, w.opts.Author, w.opts.UseCase, w.includedLabels())
	}
}

func (w *exportWizard) includedLabels() []string {
	var labels []string
	if w.opts.IncludeAgentsMD {
		labels = append(labels, "AGENTS.md")
	}
	if w.opts.IncludeSkills {
		labels = append(labels, "Skills")
	}
	if w.opts.IncludeCustomCommands {
		labels = append(labels, "Commands")
	}
	if w.opts.IncludeAgents {
		labels = append(labels, "Agents")
	}
	if w.opts.IncludeTUI {
		labels = append(labels, "TUI Config")
	}
	if w.opts.IncludeThemeFiles {
		labels = append(labels, "Custom Themes")
	}
	if w.opts.IncludePluginFiles {
		labels = append(labels, "Plugin Files")
	}
	if w.opts.IncludeToolFiles {
		labels = append(labels, "Custom Tools")
	}
	return labels
}

// RunExportWizard launches the recipe export TUI.
func RunExportWizard(wizOpts ExportWizardOptions) error {
	w := newExportWizard(wizOpts)
	p := tea.NewProgram(w, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
