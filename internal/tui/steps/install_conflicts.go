package steps

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/recipe"
)

const conflictsPageSize = 12

// InstallConflictsStep lets the user decide per-file whether to overwrite or skip
// files that already exist on disk.
type InstallConflictsStep struct {
	conflicts []recipe.Conflict
	overwrite map[string]bool // path → overwrite?
	cursor    int
	offset    int // first visible item index
}

func NewInstallConflictsStep(conflicts []recipe.Conflict) *InstallConflictsStep {
	overwrite := make(map[string]bool, len(conflicts))
	for _, c := range conflicts {
		overwrite[c.Path] = true // default: overwrite
	}
	return &InstallConflictsStep{
		conflicts: conflicts,
		overwrite: overwrite,
	}
}

// Decisions returns the AssetDecisions map for use by the installer.
func (s *InstallConflictsStep) Decisions() recipe.AssetDecisions {
	d := make(recipe.AssetDecisions, len(s.overwrite))
	for k, v := range s.overwrite {
		d[k] = v
	}
	return d
}

func (s *InstallConflictsStep) Init() tea.Cmd { return nil }

func (s *InstallConflictsStep) Update(msg tea.Msg) (Step, tea.Cmd) {
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
			if s.cursor < len(s.conflicts)-1 {
				s.cursor++
				if s.cursor >= s.offset+conflictsPageSize {
					s.offset = s.cursor - conflictsPageSize + 1
				}
			}
		case " ":
			path := s.conflicts[s.cursor].Path
			s.overwrite[path] = !s.overwrite[path]
		case "enter":
			return s, func() tea.Msg { return NextStepMsg{} }
		}
	}
	return s, nil
}

func (s *InstallConflictsStep) View() string {
	var b strings.Builder

	total := len(s.conflicts)
	b.WriteString("The following files already exist. Choose what to do with each one.\n\n")

	end := s.offset + conflictsPageSize
	if end > total {
		end = total
	}

	if s.offset > 0 {
		b.WriteString(Styles.Desc.Render(fmt.Sprintf("  ↑ %d more above\n", s.offset)))
	}

	for i := s.offset; i < end; i++ {
		c := s.conflicts[i]
		cursor := "  "
		if s.cursor == i {
			cursor = Styles.Cursor.Render("► ")
		}

		checkbox := "[ ] Skip     "
		if s.overwrite[c.Path] {
			checkbox = Styles.Selected.Render("[✓] Overwrite")
		}

		line := cursor + checkbox + "  " + Styles.Desc.Render(c.Path)
		if s.cursor == i {
			b.WriteString(Styles.Selected.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	if end < total {
		b.WriteString(Styles.Desc.Render(fmt.Sprintf("  ↓ %d more below\n", total-end)))
	}

	return b.String()
}

func (s *InstallConflictsStep) Name() string { return "File Conflicts" }

func (s *InstallConflictsStep) Info() StepInfo {
	return StepInfo{
		Name: "File Conflicts",
		KeyBindings: []KeyBinding{
			BindingsNavigate,
			BindingsSelect,
			BindingsConfirm,
			BindingsBack,
			BindingsQuit,
		},
	}
}
