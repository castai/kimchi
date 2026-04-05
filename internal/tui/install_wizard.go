package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/config"
	"github.com/castai/kimchi/internal/recipe"
	"github.com/castai/kimchi/internal/tui/steps"
)

// InstallWizardOptions are the options for the install wizard, set from CLI flags.
type InstallWizardOptions struct {
	Source  string // file path, recipe name, or name@version
	NoApply bool   // preview only; do not write any files
}

// installWizard drives the recipe install flow.
// Steps are injected dynamically after the source step resolves the recipe.
type installWizard struct {
	stepList []steps.Step
	current  int
	aborted  bool
	finished bool
	noApply  bool

	// collected across steps
	parsedRecipe *recipe.Recipe
	apiKey       string
	secretValues map[string]string // placeholder → real value, built up incrementally
	decisions    recipe.AssetDecisions

	// typed refs for dynamic injection and result collection
	sourceStep      *steps.InstallSourceStep
	secretsStep     *steps.InstallSecretsStep
	progressStep    *steps.InstallProgressStep
}

func newInstallWizard(wizOpts InstallWizardOptions) *installWizard {
	source := steps.NewInstallSourceStep(strings.TrimSpace(wizOpts.Source))
	w := &installWizard{
		noApply:      wizOpts.NoApply,
		sourceStep:   source,
		secretValues: make(map[string]string),
		stepList:     []steps.Step{source},
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
		w.applyKimchiSecrets()
		w.rebuildWriteFn()

	case *steps.InstallSecretsStep:
		for k, v := range s.SecretValues() {
			w.secretValues[k] = v
		}
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

	// Auth step only if no Kimchi API key is stored yet.
	var authStep *steps.AuthStep
	if key, _ := config.GetAPIKey(); key == "" {
		authStep = steps.NewAuthStep()
	} else {
		w.apiKey = func() string { k, _ := config.GetAPIKey(); return k }()
		w.applyKimchiSecrets()
	}

	// Secrets step if the recipe contains third-party provider or MCP secrets.
	var secretsStep *steps.InstallSecretsStep
	if externalSecrets := recipe.DetectExternalSecretPlaceholders(r); len(externalSecrets) > 0 {
		secretsStep = steps.NewInstallSecretsStep(externalSecrets)
		w.secretsStep = secretsStep
	}

	// Conflict step only if any assets clash with existing files.
	var conflictsStep *steps.InstallConflictsStep
	conflicts, _ := recipe.DetectConflicts(r)
	if len(conflicts) > 0 {
		conflictsStep = steps.NewInstallConflictsStep(conflicts)
	}

	// Assemble step list starting after the source step (index 0).
	tail := []steps.Step{preview}

	// When --no-apply is set, stop after the preview — no auth, conflicts, or install.
	if w.noApply {
		w.stepList = append(w.stepList[:1], tail...)
		return
	}

	// Progress step — always last. writeFn is a placeholder until all data is known.
	itemLabels := w.buildItemLabels(r, nil)
	progress := steps.NewInstallProgressStep(itemLabels, func() error {
		return fmt.Errorf("install not ready") // replaced by rebuildWriteFn
	})
	w.progressStep = progress

	if authStep != nil {
		tail = append(tail, authStep)
	}
	if secretsStep != nil {
		tail = append(tail, secretsStep)
	}
	if conflictsStep != nil {
		tail = append(tail, conflictsStep)
	}
	tail = append(tail, progress)
	w.stepList = append(w.stepList[:1], tail...)

	// If no interactive steps are pending, writeFn can be built immediately.
	if authStep == nil && secretsStep == nil && conflictsStep == nil {
		w.rebuildWriteFn()
	}
}

// applyKimchiSecrets maps the kimchi provider's secret placeholders to the
// stored API key. Called as soon as apiKey is known (either from storage or
// after the auth step completes).
func (w *installWizard) applyKimchiSecrets() {
	if w.parsedRecipe == nil || w.parsedRecipe.Tools.OpenCode == nil {
		return
	}
	providers := w.parsedRecipe.Tools.OpenCode.Providers
	// kimchiProviderPlaceholders is unexported; use DetectExternalSecretPlaceholders
	// complement: collect ALL placeholders, subtract external ones.
	all := make(map[string]struct{})
	recipe.CollectAllSecretPlaceholders(w.parsedRecipe, all)
	external := recipe.DetectExternalSecretPlaceholders(w.parsedRecipe)
	externalSet := make(map[string]struct{}, len(external))
	for _, p := range external {
		externalSet[p] = struct{}{}
	}
	_ = providers // used implicitly via the recipe
	for p := range all {
		if _, isExternal := externalSet[p]; !isExternal {
			w.secretValues[p] = w.apiKey
		}
	}
}

// rebuildWriteFn assembles the final writeFn once all secrets and decisions are known.
func (w *installWizard) rebuildWriteFn() {
	if w.progressStep == nil || w.parsedRecipe == nil {
		return
	}
	r := w.parsedRecipe
	secretValues := w.secretValues
	decisions := w.decisions

	w.progressStep.SetWriteFn(func() error {
		if err := recipe.InstallOpenCode(r, secretValues, decisions); err != nil {
			return err
		}
		_ = recipe.RecordInstall(r.Name, r.Version, r.Cookbook) // best-effort
		return nil
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
func RunInstallWizard(wizOpts InstallWizardOptions) error {
	w := newInstallWizard(wizOpts)
	p := tea.NewProgram(w, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
