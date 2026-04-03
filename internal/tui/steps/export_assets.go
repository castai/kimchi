package steps

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// assetExists checks whether a given asset category has at least one file present
// in the user's global OpenCode config directory. Inlined here to avoid import cycles.
func assetExists(kind string) bool {
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
		entries, err := os.ReadDir(filepath.Join(base, "commands"))
		if err != nil {
			return false
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				return true
			}
		}
		return false
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
	}
	return false
}

type assetItem struct {
	id    string
	label string
	desc  string
	found bool
}

var assetDefs = []assetItem{
	{id: "agents_md", label: "Global AGENTS.md", desc: "System prompt and rules injected into every session"},
	{id: "skills", label: "Skills", desc: "Reusable on-demand instruction sets (skills/<name>/SKILL.md)"},
	{id: "custom_commands", label: "Custom Commands", desc: "Slash command templates (commands/*.md)"},
	{id: "agents", label: "Custom Agents", desc: "Per-agent system prompts with their own models (agents/*.md)"},
}

type assetProbeCompleteMsg struct {
	items []assetItem
}

// ExportAssetsStep is a checkbox list that lets the user choose which
// OpenCode markdown assets to include in the exported recipe.
type ExportAssetsStep struct {
	items    []assetItem
	selected map[string]bool
	cursor   int
	ready    bool
}

func NewExportAssetsStep() *ExportAssetsStep {
	s := &ExportAssetsStep{
		items:    make([]assetItem, len(assetDefs)),
		selected: make(map[string]bool),
	}
	copy(s.items, assetDefs)
	return s
}

func (s *ExportAssetsStep) IncludeAgentsMD() bool       { return s.selected["agents_md"] }
func (s *ExportAssetsStep) IncludeSkills() bool          { return s.selected["skills"] }
func (s *ExportAssetsStep) IncludeCustomCommands() bool  { return s.selected["custom_commands"] }
func (s *ExportAssetsStep) IncludeAgents() bool          { return s.selected["agents"] }

func (s *ExportAssetsStep) Init() tea.Cmd {
	return func() tea.Msg {
		items := make([]assetItem, len(assetDefs))
		copy(items, assetDefs)
		for i := range items {
			items[i].found = assetExists(items[i].id)
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
