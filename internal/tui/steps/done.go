package steps

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/tools"
)

type DoneParams struct {
	APIKey           string
	ToolIDs          []tools.ToolID
	ShellProfilePath string
}

type DoneStep struct {
	toolIDs          []tools.ToolID
	shellProfilePath string
}

func NewDoneStep(params DoneParams) *DoneStep {
	return &DoneStep{
		toolIDs:          params.ToolIDs,
		shellProfilePath: params.ShellProfilePath,
	}
}

func (s *DoneStep) Init() tea.Cmd {
	return nil
}

func (s *DoneStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "enter":
			return s, tea.Quit
		}
	}
	return s, nil
}

func (s *DoneStep) View() string {
	var b strings.Builder

	b.WriteString(Styles.Title.Render("Setup Complete"))
	b.WriteString("\n\n")

	if len(s.toolIDs) > 0 {
		for _, toolID := range s.toolIDs {
			if tool, ok := tools.ByID(toolID); ok {
				tip := s.getToolTip(toolID)
				b.WriteString(fmt.Sprintf("  %s %s — %s\n", Styles.Success.Render("✓"), tool.Name, tip))
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(Styles.Help.Render("Press Enter to exit"))

	return b.String()
}


func (s *DoneStep) getToolTip(toolID tools.ToolID) string {
	switch toolID {
	case tools.ToolOpenCode:
		return "Run 'opencode' in any project directory to start. Use Ctrl+K for quick actions."
	case tools.ToolZed:
		return "Open Zed and use Cmd+Enter to send prompts to the AI assistant."
	case tools.ToolCodex:
		if s.shellProfilePath != "" {
			return fmt.Sprintf("Run 'codex' with a prompt. %s was added to %s — restart your shell or run 'source %s'.", tools.APIKeyEnv, s.shellProfilePath, s.shellProfilePath)
		}
		return fmt.Sprintf("Run 'codex' with a prompt. Ensure %s is set in your environment.", tools.APIKeyEnv)
	case tools.ToolCline:
		return "Open VS Code with Cline extension installed and start a new task."
	case tools.ToolGeneric:
		return "Source the exported environment variables in your shell."
	default:
		return "Check the tool's documentation for getting started."
	}
}


func (s *DoneStep) Name() string {
	return "Done"
}

func (s *DoneStep) Info() StepInfo {
	return StepInfo{
		Name: "Done",
		KeyBindings: []KeyBinding{
			{Key: "↵", Text: "exit"},
		},
	}
}
