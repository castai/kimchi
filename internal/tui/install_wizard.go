package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/config"
	"github.com/castai/kimchi/internal/recipe"
	"github.com/castai/kimchi/internal/tui/steps"
)

// installWizard drives the recipe install flow.
// Steps are injected dynamically after the source step resolves the recipe.
type installWizard struct {
	stepList []steps.Step
	current  int
	aborted  bool
	finished bool

	// collected across steps
	parsedRecipe *recipe.Recipe
	apiKey       string
	decisions    recipe.AssetDecisions

	// typed refs for dynamic injection and result collection
	sourceStep   *steps.InstallSourceStep
	progressStep *steps.InstallProgressStep
}

func newInstallWizard(filePath string) *installWizard {
	source := steps.NewInstallSourceStep(filePath)
	w := &installWizard{
		sourceStep: source,
		stepList:   []steps.Step{source},
	}
	return w
}

func (w *installWizard) Init() tea.Cmd {
	return tea.Batch(w.stepList[0].Init(), tea.EnterAltScreen)
}

func (w *installWizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (w *installWizard) View() string {
	if w.current >= len(w.stepList) {
		return ""
	}
	step := w.stepList[w.current]
	return steps.StepView(step.Info(), step.View())
}

func (w *installWizard) collectStepResult() {
	if w.current >= len(w.stepList) {
		return
	}
	switch s := w.stepList[w.current].(type) {
	case *steps.InstallSourceStep:
		w.parsedRecipe = s.ParsedRecipe()
		w.injectRemainingSteps()

	case *steps.AuthStep:
		w.apiKey = s.APIKey()
		w.rebuildWriteFn()

	case *steps.InstallConflictsStep:
		w.decisions = s.Decisions()
		w.rebuildWriteFn()
	}
}

// injectRemainingSteps is called once after the source step resolves the recipe.
// It builds the remaining step list based on what's needed.
func (w *installWizard) injectRemainingSteps() {
	r := w.parsedRecipe

	// Preview is always shown.
	preview := steps.NewInstallPreviewStep(r)

	// Auth step only if no API key is stored yet.
	var authStep *steps.AuthStep
	if key, _ := config.GetAPIKey(); key == "" {
		authStep = steps.NewAuthStep()
	} else {
		w.apiKey = func() string { k, _ := config.GetAPIKey(); return k }()
	}

	// Conflict step only if any assets clash with existing files.
	var conflictsStep *steps.InstallConflictsStep
	conflicts, _ := recipe.DetectConflicts(r)
	if len(conflicts) > 0 {
		conflictsStep = steps.NewInstallConflictsStep(conflicts)
	}

	// Progress step — always last. writeFn is a placeholder until decisions are finalized.
	itemLabels := w.buildItemLabels(r, nil)
	progress := steps.NewInstallProgressStep(itemLabels, func() error {
		return fmt.Errorf("install not ready") // replaced by rebuildWriteFn
	})
	w.progressStep = progress

	// Assemble step list starting after the source step (index 0).
	tail := []steps.Step{preview}
	if authStep != nil {
		tail = append(tail, authStep)
	}
	if conflictsStep != nil {
		tail = append(tail, conflictsStep)
	}
	tail = append(tail, progress)
	w.stepList = append(w.stepList[:1], tail...)

	// If neither auth nor conflicts are needed, writeFn can be built now.
	if authStep == nil && conflictsStep == nil {
		w.rebuildWriteFn()
	}
}

// rebuildWriteFn assembles the final writeFn once both apiKey and decisions are known.
func (w *installWizard) rebuildWriteFn() {
	if w.progressStep == nil || w.parsedRecipe == nil {
		return
	}
	// decisions may still be nil (no conflicts) — InstallOpenCode handles nil map.
	r := w.parsedRecipe
	apiKey := w.apiKey
	decisions := w.decisions

	w.progressStep.SetWriteFn(func() error {
		return recipe.InstallOpenCode(r, apiKey, decisions)
	})
}

// buildItemLabels returns human-readable labels for the progress step checklist.
func (w *installWizard) buildItemLabels(r *recipe.Recipe, _ recipe.AssetDecisions) []string {
	oc := r.Tools.OpenCode
	labels := []string{"opencode.json"}

	if oc == nil {
		return labels
	}
	if oc.TUI != nil {
		labels = append(labels, "tui.json")
	}
	if oc.AgentsMD != "" {
		labels = append(labels, "AGENTS.md")
	}
	for _, s := range oc.Skills {
		labels = append(labels, fmt.Sprintf("skills/%s/SKILL.md", s.Name))
		for _, f := range s.Files {
			labels = append(labels, fmt.Sprintf("skills/%s/%s", s.Name, f.Path))
		}
	}
	for _, c := range oc.CustomCommands {
		labels = append(labels, fmt.Sprintf("commands/%s.md", c.Name))
	}
	for _, a := range oc.Agents {
		labels = append(labels, fmt.Sprintf("agents/%s.md", a.Name))
	}
	for _, f := range oc.ThemeFiles {
		labels = append(labels, fmt.Sprintf("themes/%s", f.Path))
	}
	for _, f := range oc.PluginFiles {
		labels = append(labels, fmt.Sprintf("plugins/%s", f.Path))
	}
	for _, f := range oc.ToolFiles {
		labels = append(labels, fmt.Sprintf("tools/%s", f.Path))
	}
	for _, f := range oc.ReferencedFiles {
		labels = append(labels, f.Path)
	}
	return labels
}

// RunInstallWizard launches the recipe install TUI.
// filePath may be empty — the wizard will prompt for it.
func RunInstallWizard(filePath string) error {
	w := newInstallWizard(strings.TrimSpace(filePath))
	p := tea.NewProgram(w, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
