package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/config"
	"github.com/castai/kimchi/internal/recipe"
	"github.com/castai/kimchi/internal/tools"
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
	parsedRecipe   *recipe.Recipe
	filteredRecipe *recipe.Recipe // recipe after user's asset selection
	apiKey         string
	secretValues   map[string]string // placeholder → real value, built up incrementally
	decisions      recipe.AssetDecisions

	// typed refs for dynamic injection and result collection
	sourceStep   *steps.InstallSourceStep
	assetsStep   *steps.InstallAssetsStep
	secretsStep  *steps.InstallSecretsStep
	progressStep *steps.InstallProgressStep
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
		w.filteredRecipe = w.parsedRecipe
		w.injectRemainingSteps()

	case *steps.InstallAssetsStep:
		w.filteredRecipe = s.FilteredRecipe()
		w.applyKimchiSecrets()
		w.rebuildWriteFn()

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

	// Assets selection step — always shown so the user can pick a subset.
	assetsStep := steps.NewInstallAssetsStep(r)
	w.assetsStep = assetsStep

	// Assemble step list starting after the source step (index 0).
	tail := []steps.Step{preview, assetsStep}

	// When --no-apply is set, stop after the preview — no auth, conflicts, or install.
	if w.noApply {
		w.stepList = append(w.stepList[:1], tail...)
		return
	}

	// Auth step only if no Kimchi API key is stored yet.
	// NOTE: auth / secrets / conflicts are based on the full recipe at this point;
	// they are re-evaluated after the assets step in rebuildWriteFn.
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
	tail = append(tail, progress)
	w.stepList = append(w.stepList[:1], tail...)
}

// applyKimchiSecrets maps the kimchi provider's secret placeholders to the
// stored API key. Called as soon as apiKey is known (either from storage or
// after the auth step completes).
func (w *installWizard) applyKimchiSecrets() {
	r := w.filteredRecipe
	if r == nil {
		r = w.parsedRecipe
	}
	if r == nil || r.Tools.OpenCode == nil {
		return
	}
	// Collect ALL placeholders, subtract external ones — the remainder are kimchi secrets.
	all := make(map[string]struct{})
	recipe.CollectAllSecretPlaceholders(r, all)
	external := recipe.DetectExternalSecretPlaceholders(r)
	externalSet := make(map[string]struct{}, len(external))
	for _, p := range external {
		externalSet[p] = struct{}{}
	}
	for p := range all {
		if _, isExternal := externalSet[p]; !isExternal {
			w.secretValues[p] = w.apiKey
		}
	}
}

// rebuildWriteFn assembles the final writeFn once all secrets and decisions are known.
func (w *installWizard) rebuildWriteFn() {
	if w.progressStep == nil {
		return
	}
	r := w.filteredRecipe
	if r == nil {
		r = w.parsedRecipe
	}
	if r == nil {
		return
	}
	// Update the progress checklist to reflect the filtered asset selection.
	w.progressStep.SetItems(w.buildItemLabels(r, nil))

	secretValues := w.secretValues
	decisions := w.decisions
	orig := w.parsedRecipe // for RecordInstall (name/version/cookbook from original)

	w.progressStep.SetWriteFn(func() error {
		filesToCapture, err := recipe.PredictAssetPaths(orig)
		if err != nil {
			return fmt.Errorf("predict asset paths: %w", err)
		}
		if err := recipe.EnsureBaseline(tools.ToolOpenCode, filesToCapture); err != nil {
			return fmt.Errorf("backup baseline: %w", err)
		}
		if err := recipe.SnapshotCurrentlyInstalled(tools.ToolOpenCode); err != nil {
			return fmt.Errorf("backup current recipes: %w", err)
		}
		if err := recipe.UninstallByManifest(tools.ToolOpenCode, orig.Name); err != nil {
			return fmt.Errorf("uninstall prior recipe: %w", err)
		}

		written, err := recipe.InstallOpenCode(r, secretValues, decisions)
		if err != nil {
			return err
		}

		// Save manifest (exclude opencode.json — merge target, not verbatim).
		var assetFiles []string
		for _, p := range written {
			if filepath.Base(p) != "opencode.json" {
				assetFiles = append(assetFiles, p)
			}
		}
		_ = recipe.SaveManifest(&recipe.RecipeManifest{
			RecipeName:  orig.Name,
			Tool:        tools.ToolOpenCode,
			InstalledAt: time.Now(),
			AssetFiles:  assetFiles,
		})
		_ = recipe.RecordInstall(orig.Name, orig.Version, orig.Cookbook, tools.ToolOpenCode)
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
