package steps

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/config"
)

// assetDef describes one selectable asset category.
type assetDef struct {
	id         string
	label      string
	desc       string
	globalOnly bool // if true, hide when scope is project
}

var assetDefs = []assetDef{
	{id: "agents_md", label: "AGENTS.md", desc: "System prompt and rules injected into every session"},
	{id: "skills", label: "Skills", desc: "Reusable on-demand instruction sets (skills/<name>/SKILL.md)"},
	{id: "custom_commands", label: "Custom Commands", desc: "Slash command templates (commands/**/*.md)"},
	{id: "agents", label: "Custom Agents", desc: "Per-agent system prompts with their own models (agents/*.md)"},
	{id: "tui", label: "TUI Config", desc: "Theme, keybinds and display settings (tui.json)", globalOnly: true},
	{id: "theme_files", label: "Custom Themes", desc: "Custom theme JSON files (themes/*.json)", globalOnly: true},
	{id: "plugin_files", label: "Plugin Files", desc: "Custom plugin source files (plugins/)", globalOnly: true},
	{id: "tool_files", label: "Custom Tools", desc: "Custom tool definitions (tools/)", globalOnly: true},
}

type assetItem struct {
	assetDef
	found bool
}

// assetExistsForScope checks whether an asset category exists, searching the
// correct paths for the given scope.
func assetExistsForScope(kind string, scope config.ConfigScope) bool {
	switch scope {
	case config.ScopeProject:
		return assetExistsProject(kind)
	default:
		return assetExistsGlobal(kind)
	}
}

func assetExistsGlobal(kind string) bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	base := filepath.Join(homeDir, ".config", "opencode")
	switch kind {
	case "agents_md":
		_, err := os.Stat(filepath.Join(base, "AGENTS.md"))
		return err == nil
	case "skills":
		entries, err := os.ReadDir(filepath.Join(base, "skills"))
		return err == nil && len(entries) > 0
	case "custom_commands":
		return dirContainsMarkdown(filepath.Join(base, "commands"))
	case "agents":
		entries, err := os.ReadDir(filepath.Join(base, "agents"))
		if err != nil {
			return false
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				return true
			}
		}
		return false
	case "tui":
		_, err := os.Stat(filepath.Join(base, "tui.json"))
		return err == nil
	case "theme_files":
		return dirContainsExt(filepath.Join(base, "themes"), ".json")
	case "plugin_files":
		return dirHasFiles(filepath.Join(base, "plugins"))
	case "tool_files":
		return dirHasFiles(filepath.Join(base, "tools"))
	}
	return false
}

func assetExistsProject(kind string) bool {
	cwd, err := os.Getwd()
	if err != nil {
		return false
	}
	dotOpenCode := filepath.Join(cwd, ".opencode")
	switch kind {
	case "agents_md":
		_, err := os.Stat(filepath.Join(cwd, "AGENTS.md"))
		return err == nil
	case "skills":
		entries, err := os.ReadDir(filepath.Join(dotOpenCode, "skills"))
		return err == nil && len(entries) > 0
	case "custom_commands":
		return dirContainsMarkdown(filepath.Join(dotOpenCode, "commands"))
	case "agents":
		entries, err := os.ReadDir(filepath.Join(dotOpenCode, "agents"))
		if err != nil {
			return false
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				return true
			}
		}
		return false
	// global-only items never exist in project scope
	case "tui", "theme_files", "plugin_files", "tool_files":
		return false
	}
	return false
}

// dirContainsMarkdown reports whether dir or any of its subdirectories
// contains at least one *.md file.
func dirContainsMarkdown(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			if dirContainsMarkdown(filepath.Join(dir, e.Name())) {
				return true
			}
		} else if strings.HasSuffix(e.Name(), ".md") {
			return true
		}
	}
	return false
}

// dirContainsExt reports whether dir contains at least one file with the given extension.
func dirContainsExt(dir, ext string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ext) {
			return true
		}
	}
	return false
}

// dirHasFiles reports whether dir exists and contains at least one file (any type).
func dirHasFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			return true
		}
	}
	return false
}

type assetProbeCompleteMsg struct {
	items []assetItem
}

// ExportAssetsStep is a checkbox list that lets the user choose which
// OpenCode assets to include in the exported recipe.
type ExportAssetsStep struct {
	scope    config.ConfigScope
	items    []assetItem
	selected map[string]bool
	cursor   int
	ready    bool
}

func NewExportAssetsStep(scope config.ConfigScope) *ExportAssetsStep {
	s := &ExportAssetsStep{
		scope:    scope,
		selected: make(map[string]bool),
	}
	return s
}

func (s *ExportAssetsStep) IncludeAgentsMD() bool       { return s.selected["agents_md"] }
func (s *ExportAssetsStep) IncludeSkills() bool         { return s.selected["skills"] }
func (s *ExportAssetsStep) IncludeCustomCommands() bool { return s.selected["custom_commands"] }
func (s *ExportAssetsStep) IncludeAgents() bool         { return s.selected["agents"] }
func (s *ExportAssetsStep) IncludeTUI() bool            { return s.selected["tui"] }
func (s *ExportAssetsStep) IncludeThemeFiles() bool     { return s.selected["theme_files"] }
func (s *ExportAssetsStep) IncludePluginFiles() bool    { return s.selected["plugin_files"] }
func (s *ExportAssetsStep) IncludeToolFiles() bool      { return s.selected["tool_files"] }

func (s *ExportAssetsStep) Init() tea.Cmd {
	scope := s.scope
	return func() tea.Msg {
		var items []assetItem
		for _, def := range assetDefs {
			if def.globalOnly && scope == config.ScopeProject {
				continue
			}
			item := assetItem{assetDef: def}
			item.found = assetExistsForScope(def.id, scope)
			items = append(items, item)
		}
		return assetProbeCompleteMsg{items: items}
	}
}

func (s *ExportAssetsStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case assetProbeCompleteMsg:
		s.items = msg.items
		for _, item := range s.items {
			if item.found {
				s.selected[item.id] = true
			}
		}
		s.ready = true
		return s, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return s, func() tea.Msg { return AbortMsg{} }
		case "esc":
			return s, func() tea.Msg { return PrevStepMsg{} }
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < len(s.items)-1 {
				s.cursor++
			}
		case " ":
			id := s.items[s.cursor].id
			s.selected[id] = !s.selected[id]
		case "enter":
			return s, func() tea.Msg { return NextStepMsg{} }
		}
	}
	return s, nil
}

func (s *ExportAssetsStep) View() string {
	var b strings.Builder

	if !s.ready {
		b.WriteString(Styles.Spinner.Render("Checking for assets..."))
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString("Select which OpenCode assets to include in the recipe.\n\n")

	for i, item := range s.items {
		cursor := "  "
		if s.cursor == i {
			cursor = Styles.Cursor.Render("► ")
		}

		checkbox := "[ ]"
		if s.selected[item.id] {
			checkbox = Styles.Selected.Render("[✓]")
		}

		found := ""
		if item.found {
			found = Styles.Success.Render(" ✓ found")
		} else {
			found = Styles.Desc.Render(" (not found)")
		}

		firstLine := cursor + checkbox + " " + item.label + found
		if s.cursor == i {
			b.WriteString(Styles.Selected.Render(firstLine))
		} else {
			b.WriteString(firstLine)
		}
		b.WriteString("\n")
		b.WriteString("      " + Styles.Desc.Render(item.desc))
		b.WriteString("\n")
	}

	return b.String()
}

func (s *ExportAssetsStep) Name() string { return "Include Assets" }

func (s *ExportAssetsStep) Info() StepInfo {
	return StepInfo{
		Name: "Include Assets",
		KeyBindings: []KeyBinding{
			BindingsNavigate,
			BindingsSelect,
			BindingsConfirm,
			BindingsBack,
			BindingsQuit,
		},
	}
}
