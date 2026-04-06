package steps

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/recipe"
)

const assetsPageSize = 14

type installAssetItem struct {
	id    string // unique key: "config", "agents_md", "skill:name", etc.
	label string // display label
	desc  string // secondary line
}

// InstallAssetsStep lets the user pick which assets from a recipe to install.
// All items default to selected. FilteredRecipe() returns a copy of the recipe
// containing only the checked assets.
type InstallAssetsStep struct {
	r        *recipe.Recipe
	items    []installAssetItem
	selected map[string]bool
	cursor   int
	offset   int
}

func NewInstallAssetsStep(r *recipe.Recipe) *InstallAssetsStep {
	s := &InstallAssetsStep{
		r:        r,
		selected: make(map[string]bool),
	}
	s.buildItems()
	return s
}

func (s *InstallAssetsStep) buildItems() {
	oc := s.r.Tools.OpenCode
	if oc == nil {
		return
	}

	add := func(id, label, desc string) {
		s.items = append(s.items, installAssetItem{id: id, label: label, desc: desc})
		s.selected[id] = true
	}

	add("config", "Config settings", "Model, providers, MCP servers, and other opencode.json settings")

	if oc.AgentsMD != "" {
		add("agents_md", "AGENTS.md", "System prompt injected into every session")
	}
	for _, sk := range oc.Skills {
		add("skill:"+sk.Name, fmt.Sprintf("Skill: %s", sk.Name), "skills/"+sk.Name+"/SKILL.md")
	}
	for _, c := range oc.CustomCommands {
		add("command:"+c.Name, fmt.Sprintf("Command: %s", c.Name), "commands/"+c.Name+".md")
	}
	for _, a := range oc.Agents {
		add("agent:"+a.Name, fmt.Sprintf("Agent: %s", a.Name), "agents/"+a.Name+".md")
	}
	if oc.TUI != nil {
		add("tui", "TUI config", "Theme, keybinds and display settings (tui.json)")
	}
	for _, f := range oc.ThemeFiles {
		add("theme:"+f.Path, fmt.Sprintf("Theme: %s", f.Path), "themes/"+f.Path)
	}
	for _, f := range oc.PluginFiles {
		add("plugin:"+f.Path, fmt.Sprintf("Plugin: %s", f.Path), "plugins/"+f.Path)
	}
	for _, f := range oc.ToolFiles {
		add("tool:"+f.Path, fmt.Sprintf("Tool: %s", f.Path), "tools/"+f.Path)
	}
}

// FilteredRecipe returns a shallow copy of the recipe with only the selected assets.
func (s *InstallAssetsStep) FilteredRecipe() *recipe.Recipe {
	src := s.r
	oc := src.Tools.OpenCode
	if oc == nil {
		return src
	}

	filtered := *oc // shallow copy of OpenCodeConfig

	// Strip config settings if deselected.
	if !s.selected["config"] {
		filtered.Providers = nil
		filtered.Model = ""
		filtered.SmallModel = ""
		filtered.DefaultAgent = ""
		filtered.DisabledProviders = nil
		filtered.EnabledProviders = nil
		filtered.Plugin = nil
		filtered.Snapshot = nil
		filtered.Instructions = nil
		filtered.Compaction = nil
		filtered.AgentConfigs = nil
		filtered.MCP = nil
		filtered.Permission = nil
		filtered.Tools = nil
		filtered.Experimental = nil
		filtered.Formatter = nil
		filtered.LSP = nil
		filtered.InlineCommands = nil
	}

	if !s.selected["agents_md"] {
		filtered.AgentsMD = ""
	}

	var skills []recipe.SkillEntry
	for _, sk := range oc.Skills {
		if s.selected["skill:"+sk.Name] {
			skills = append(skills, sk)
		}
	}
	filtered.Skills = skills

	var commands []recipe.CommandEntry
	for _, c := range oc.CustomCommands {
		if s.selected["command:"+c.Name] {
			commands = append(commands, c)
		}
	}
	filtered.CustomCommands = commands

	var agents []recipe.AgentEntry
	for _, a := range oc.Agents {
		if s.selected["agent:"+a.Name] {
			agents = append(agents, a)
		}
	}
	filtered.Agents = agents

	if !s.selected["tui"] {
		filtered.TUI = nil
	}

	var themes []recipe.FileEntry
	for _, f := range oc.ThemeFiles {
		if s.selected["theme:"+f.Path] {
			themes = append(themes, f)
		}
	}
	filtered.ThemeFiles = themes

	var plugins []recipe.FileEntry
	for _, f := range oc.PluginFiles {
		if s.selected["plugin:"+f.Path] {
			plugins = append(plugins, f)
		}
	}
	filtered.PluginFiles = plugins

	var toolFiles []recipe.FileEntry
	for _, f := range oc.ToolFiles {
		if s.selected["tool:"+f.Path] {
			toolFiles = append(toolFiles, f)
		}
	}
	filtered.ToolFiles = toolFiles

	// Keep only referenced files that are referenced by selected markdown assets.
	hasAnyMD := !s.selected["agents_md"] && filtered.AgentsMD == "" // already cleared above
	_ = hasAnyMD
	// Simplest safe approach: include referenced files only when at least one
	// markdown asset is selected.
	anyMD := filtered.AgentsMD != "" || len(filtered.Skills) > 0 ||
		len(filtered.CustomCommands) > 0 || len(filtered.Agents) > 0
	if !anyMD {
		filtered.ReferencedFiles = nil
	}

	r := *src
	r.Tools.OpenCode = &filtered
	return &r
}

func (s *InstallAssetsStep) Init() tea.Cmd { return nil }

func (s *InstallAssetsStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return s, func() tea.Msg { return AbortMsg{} }
		case "esc":
			return s, func() tea.Msg { return PrevStepMsg{} }
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
				if s.cursor < s.offset {
					s.offset = s.cursor
				}
			}
		case "down", "j":
			if s.cursor < len(s.items)-1 {
				s.cursor++
				if s.cursor >= s.offset+assetsPageSize {
					s.offset = s.cursor - assetsPageSize + 1
				}
			}
		case " ":
			id := s.items[s.cursor].id
			s.selected[id] = !s.selected[id]
		case "a":
			// Toggle all on/off.
			allOn := true
			for _, item := range s.items {
				if !s.selected[item.id] {
					allOn = false
					break
				}
			}
			for _, item := range s.items {
				s.selected[item.id] = !allOn
			}
		case "enter":
			return s, func() tea.Msg { return NextStepMsg{} }
		}
	}
	return s, nil
}

func (s *InstallAssetsStep) View() string {
	var b strings.Builder

	if len(s.items) == 0 {
		b.WriteString("No selectable assets in this recipe.\n")
		b.WriteString(Styles.Help.Render("Press enter to continue"))
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString("Choose which assets to install from this recipe.\n\n")

	total := len(s.items)
	end := s.offset + assetsPageSize
	if end > total {
		end = total
	}

	if s.offset > 0 {
		b.WriteString(Styles.Desc.Render(fmt.Sprintf("  ↑ %d more above\n", s.offset)))
	}

	for i := s.offset; i < end; i++ {
		item := s.items[i]
		cursor := "  "
		if s.cursor == i {
			cursor = Styles.Cursor.Render("► ")
		}

		checkbox := "[ ]"
		if s.selected[item.id] {
			checkbox = Styles.Selected.Render("[✓]")
		}

		firstLine := cursor + checkbox + " " + item.label
		if s.cursor == i {
			b.WriteString(Styles.Selected.Render(firstLine))
		} else {
			b.WriteString(firstLine)
		}
		b.WriteString("\n")
		b.WriteString("      " + Styles.Desc.Render(item.desc) + "\n")
	}

	if end < total {
		b.WriteString(Styles.Desc.Render(fmt.Sprintf("  ↓ %d more below\n", total-end)))
	}

	return b.String()
}

func (s *InstallAssetsStep) Name() string { return "Select Assets" }

func (s *InstallAssetsStep) Info() StepInfo {
	return StepInfo{
		Name: "Select Assets",
		KeyBindings: []KeyBinding{
			BindingsNavigate,
			BindingsSelect,
			{Key: "a", Text: "toggle all"},
			BindingsConfirm,
			BindingsBack,
			BindingsQuit,
		},
	}
}
