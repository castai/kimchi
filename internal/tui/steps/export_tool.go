package steps

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/tools"
)

// exportableTools lists the tool IDs that support recipe export.
// Extend this slice when more tools are supported.
var exportableTools = []tools.ToolID{
	tools.ToolOpenCode,
}

// ExportToolStep is a radio-select step that shows only the exportable tools
// that are actually installed on the system.
type ExportToolStep struct {
	available []tools.Tool
	selected  int
}

func NewExportToolStep() *ExportToolStep {
	var available []tools.Tool
	for _, id := range exportableTools {
		if t, ok := tools.ByID(id); ok && t.DetectInstalled() {
			available = append(available, t)
		}
	}
	return &ExportToolStep{available: available}
}

// HasTools reports whether at least one exportable tool was found.
func (s *ExportToolStep) HasTools() bool { return len(s.available) > 0 }

// SelectedTool returns the ToolID chosen by the user, or empty string if none.
func (s *ExportToolStep) SelectedTool() tools.ToolID {
	if len(s.available) == 0 {
		return ""
	}
	return s.available[s.selected].ID
}

func (s *ExportToolStep) Init() tea.Cmd { return nil }

func (s *ExportToolStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return s, func() tea.Msg { return AbortMsg{} }
		case "esc":
			return s, func() tea.Msg { return PrevStepMsg{} }
		case "up", "k":
			if s.selected > 0 {
				s.selected--
			}
		case "down", "j":
			if s.selected < len(s.available)-1 {
				s.selected++
			}
		case "enter":
			if len(s.available) == 0 {
				return s, func() tea.Msg { return AbortMsg{} }
			}
			return s, func() tea.Msg { return NextStepMsg{} }
		}
	}
	return s, nil
}

func (s *ExportToolStep) View() string {
	var b strings.Builder

	if len(s.available) == 0 {
		b.WriteString(Styles.Warning.Render("No supported tools detected."))
		b.WriteString("\n\n")
		b.WriteString("Recipe export currently supports: ")
		names := make([]string, 0, len(exportableTools))
		for _, id := range exportableTools {
			if t, ok := tools.ByID(id); ok {
				names = append(names, t.Name)
			}
		}
		b.WriteString(strings.Join(names, ", "))
		b.WriteString("\n\n")
		b.WriteString(Styles.Desc.Render("Install one of the tools above and run kimchi recipe export again."))
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString("Select the tool to export a recipe for.\n\n")

	for i, t := range s.available {
		cursor := "  "
		if s.selected == i {
			cursor = Styles.Cursor.Render("► ")
		}
		radio := "○"
		if s.selected == i {
			radio = Styles.Selected.Render("●")
		}
		line := fmt.Sprintf("%s %s %-12s  %s", cursor, radio, t.Name, Styles.Desc.Render(t.Description))
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

func (s *ExportToolStep) Name() string { return "Select Tool" }

func (s *ExportToolStep) Info() StepInfo {
	bindings := []KeyBinding{BindingsQuit}
	if len(s.available) > 0 {
		bindings = []KeyBinding{BindingsNavigate, BindingsConfirm, BindingsBack, BindingsQuit}
	}
	return StepInfo{
		Name:        "Select Tool",
		KeyBindings: bindings,
	}
}
