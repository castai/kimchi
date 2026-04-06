package steps

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/recipe"
)

// InstallPreviewStep shows a summary of the parsed recipe and asks the user to confirm.
type InstallPreviewStep struct {
	r *recipe.Recipe
}

func NewInstallPreviewStep(r *recipe.Recipe) *InstallPreviewStep {
	return &InstallPreviewStep{r: r}
}

func (s *InstallPreviewStep) Init() tea.Cmd { return nil }

func (s *InstallPreviewStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return s, func() tea.Msg { return AbortMsg{} }
		case "esc":
			return s, func() tea.Msg { return PrevStepMsg{} }
		case "enter":
			return s, func() tea.Msg { return NextStepMsg{} }
		}
	}
	return s, nil
}

func (s *InstallPreviewStep) View() string {
	var b strings.Builder
	r := s.r
	oc := r.Tools.OpenCode

	b.WriteString("Review the recipe before installing.\n\n")

	b.WriteString(fmt.Sprintf("  %s %s\n", Styles.Desc.Render("Name:    "), r.Name))
	if r.Author != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", Styles.Desc.Render("Author:  "), r.Author))
	}
	if r.Description != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", Styles.Desc.Render("Desc:    "), r.Description))
	}
	b.WriteString(fmt.Sprintf("  %s %s\n", Styles.Desc.Render("Tool:    "), "OpenCode"))
	b.WriteString(fmt.Sprintf("  %s %s\n", Styles.Desc.Render("Use case:"), r.UseCase))
	b.WriteString(fmt.Sprintf("  %s %s\n", Styles.Desc.Render("Model:   "), r.Model))

	if oc != nil && hasAssets(oc) {
		b.WriteString("\n  " + Styles.Desc.Render("Assets included:") + "\n")
		if oc.AgentsMD != "" {
			b.WriteString(Styles.Success.Render("    ✓ AGENTS.md") + "\n")
		}
		if len(oc.Skills) > 0 {
			b.WriteString(Styles.Success.Render(fmt.Sprintf("    ✓ %d skill(s)", len(oc.Skills))) + "\n")
		}
		if len(oc.CustomCommands) > 0 {
			b.WriteString(Styles.Success.Render(fmt.Sprintf("    ✓ %d custom command(s)", len(oc.CustomCommands))) + "\n")
		}
		if len(oc.Agents) > 0 {
			b.WriteString(Styles.Success.Render(fmt.Sprintf("    ✓ %d custom agent(s)", len(oc.Agents))) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(Styles.Help.Render("Press enter to continue, esc to go back"))
	b.WriteString("\n")

	return b.String()
}

func hasAssets(oc *recipe.OpenCodeConfig) bool {
	return oc.AgentsMD != "" || len(oc.Skills) > 0 || len(oc.CustomCommands) > 0 || len(oc.Agents) > 0
}

func (s *InstallPreviewStep) Name() string { return "Preview" }

func (s *InstallPreviewStep) Info() StepInfo {
	return StepInfo{
		Name:        "Recipe Preview",
		KeyBindings: []KeyBinding{BindingsConfirm, BindingsBack, BindingsQuit},
	}
}
