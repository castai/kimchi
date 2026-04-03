package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/recipe"
	"github.com/castai/kimchi/internal/tools"
	"github.com/castai/kimchi/internal/tui/steps"
)

const defaultOutputPath = "kimchi-recipe.yaml"

// exportWizard is a standalone bubbletea model for the recipe export flow.
type exportWizard struct {
	stepList     []steps.Step
	current      int
	opts         recipe.ExportOptions
	finished     bool
	aborted      bool
	outputPath   string
	selectedTool tools.ToolID

	// typed references for result collection
	toolStep    *steps.ExportToolStep
	metaStep    *steps.ExportMetaStep
	useCaseStep *steps.ExportUseCaseStep
	assetsStep  *steps.ExportAssetsStep
	outputStep  *steps.ExportOutputStep // nil when --output flag was provided
	confirmStep *steps.ExportConfirmStep
}

// newExportWizard builds the wizard. If outputPath is non-empty the output
// file step is skipped and that path is used directly.
func newExportWizard(outputPath string) *exportWizard {
	toolStep := steps.NewExportToolStep()
	meta := steps.NewExportMetaStep()
	useCase := steps.NewExportUseCaseStep()
	assets := steps.NewExportAssetsStep()

	w := &exportWizard{
		outputPath:  outputPath,
		toolStep:    toolStep,
		metaStep:    meta,
		useCaseStep: useCase,
		assetsStep:  assets,
	}

	// writeFn is called by the confirm step when the user presses Enter.
	// It reads opts, selectedTool, and outputPath from w at that point in time.
	writeFn := func() error {
		switch w.selectedTool {
		case tools.ToolOpenCode:
			a, err := recipe.ReadOpenCodeAssets()
			if err != nil {
				return fmt.Errorf("read opencode assets: %w", err)
			}
			r, err := recipe.Build(a, w.opts)
			if err != nil {
				return fmt.Errorf("build recipe: %w", err)
			}
			return recipe.WriteYAML(w.outputPath, r)
		default:
			return fmt.Errorf("unsupported tool: %s", w.selectedTool)
		}
	}

	// Placeholder path for the confirm step — updated later in collectStepResult.
	confirmOutputPath := outputPath
	if confirmOutputPath == "" {
		confirmOutputPath = defaultOutputPath
	}
	confirm := steps.NewExportConfirmStep(confirmOutputPath, writeFn, "", "", "", nil)
	w.confirmStep = confirm

	stepList := []steps.Step{toolStep, meta, useCase, assets}
	if outputPath == "" {
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
		if w.outputStep == nil {
			// --output was provided; summary is complete now.
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
	return labels
}

// RunExportWizard launches the recipe export TUI and writes to outputPath.
func RunExportWizard(outputPath string) error {
	w := newExportWizard(outputPath)
	p := tea.NewProgram(w, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
