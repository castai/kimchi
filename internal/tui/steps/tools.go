package steps

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/tools"
)

type ToolsStep struct {
	toolList  []tools.Tool
	selected  map[tools.ToolID]bool
	cursor    int
	showError bool
}

func NewToolsStep() *ToolsStep {
	toolList := tools.All()
	selected := make(map[tools.ToolID]bool)

	for _, tool := range toolList {
		if tool.DetectInstalled() {
			selected[tool.ID] = true
		}
	}

	return &ToolsStep{
		toolList: toolList,
		selected: selected,
		cursor:   0,
	}
}

func (s *ToolsStep) SelectedTools() []tools.ToolID {
	var result []tools.ToolID
	for _, t := range s.toolList {
		if s.selected[t.ID] {
			result = append(result, t.ID)
		}
	}
	return result
}

func (s *ToolsStep) Init() tea.Cmd {
	return nil
}

func (s *ToolsStep) Update(msg tea.Msg) (Step, tea.Cmd) {
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
			}
		case "down", "j":
			if s.cursor < len(s.toolList)-1 {
				s.cursor++
			}
		case " ":
			tool := s.toolList[s.cursor]
			s.selected[tool.ID] = !s.selected[tool.ID]
			if len(s.SelectedTools()) > 0 {
				s.showError = false
			}
		case "enter":
			if len(s.SelectedTools()) == 0 {
				s.showError = true
				return s, nil
			}
			return s, func() tea.Msg { return NextStepMsg{} }
		}
	}
	return s, nil
}

func (s *ToolsStep) View() string {
	var b strings.Builder

	b.WriteString(Styles.Title.Render("Select AI tools to configure"))
	b.WriteString("\n\n")

	for i, tool := range s.toolList {
		cursor := "  "
		if s.cursor == i {
			cursor = Styles.Cursor.Render("►")
		}

		checkbox := "[ ]"
		if s.selected[tool.ID] {
			checkbox = Styles.Selected.Render("[✓]")
		}

		installed := ""
		if tool.DetectInstalled() {
			installed = Styles.Success.Render(" ✓ installed")
		}

		desc := Styles.Desc.Render(tool.Description)
		line := fmt.Sprintf("%s %s %-12s %s%s", cursor, checkbox, tool.Name, desc, installed)

		if s.cursor == i {
			b.WriteString(Styles.Selected.Render(line))
		} else {
			b.WriteString(Styles.Item.Render(line))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	if s.showError {
		b.WriteString(Styles.Error.Render("✗ Please select at least one tool"))
		b.WriteString("\n")
	}

	return b.String()
}

func (s *ToolsStep) Name() string {
	return "Select Tools"
}

func (s *ToolsStep) Info() StepInfo {
	return StepInfo{
		Name: "Select Tools",
		KeyBindings: []KeyBinding{
			BindingsNavigate,
			BindingsSelect,
			BindingsConfirm,
			BindingsBack,
			BindingsQuit,
		},
	}
}
