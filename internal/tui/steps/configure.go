package steps

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/config"
	"github.com/castai/kimchi/internal/tools"
)

type toolStatus struct {
	tool    tools.Tool
	status  string
	err     error
	writing bool
}

type ConfigureStep struct {
	toolIDs        []tools.ToolID
	scope          config.ConfigScope
	telemetryOptIn bool
	statuses       []toolStatus
	done           bool
	startOnce      sync.Once
}

type writeCompleteMsg struct {
	index  int
	status string
	err    error
}

type startWriteMsg struct{}

func NewConfigureStep(toolIDs []tools.ToolID, scope config.ConfigScope, telemetryOptIn bool) *ConfigureStep {
	return &ConfigureStep{
		toolIDs:        toolIDs,
		scope:          scope,
		telemetryOptIn: telemetryOptIn,
		statuses:       make([]toolStatus, len(toolIDs)),
	}
}

func (s *ConfigureStep) Init() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return startWriteMsg{}
	})
}

func (s *ConfigureStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch m := msg.(type) {
	case startWriteMsg:
		s.startOnce.Do(func() {
			for i, toolID := range s.toolIDs {
				tool, ok := tools.ByID(toolID)
				if !ok {
					s.statuses[i] = toolStatus{tool: tool, status: "unknown tool", err: fmt.Errorf("unknown tool: %s", toolID)}
					continue
				}
				s.statuses[i] = toolStatus{tool: tool, writing: true}
			}
		})
		var cmds []tea.Cmd
		for i, status := range s.statuses {
			if status.writing && status.err == nil && status.status == "" {
				cmds = append(cmds, s.writeToolConfig(i))
			}
		}
		if len(cmds) > 0 {
			return s, tea.Batch(cmds...)
		}
		if s.allComplete() {
			s.done = true
			return s, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
				return NextStepMsg{}
			})
		}
		return s, nil

	case writeCompleteMsg:
		s.statuses[m.index] = toolStatus{
			tool:   s.statuses[m.index].tool,
			status: m.status,
			err:    m.err,
		}
		if s.allComplete() {
			s.done = true
			if s.hasErrors() {
				return s, nil
			}
			return s, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
				return NextStepMsg{}
			})
		}
		return s, nil

	case tea.KeyMsg:
		if s.done && s.hasErrors() {
			switch msg.(tea.KeyMsg).String() {
			case "enter", "ctrl+c", "q":
				return s, func() tea.Msg { return NextStepMsg{} }
			}
		}
	}

	return s, nil
}

func (s *ConfigureStep) writeToolConfig(index int) tea.Cmd {
	return func() tea.Msg {
		tool := s.statuses[index].tool
		if tool.Write == nil {
			return writeCompleteMsg{index: index, status: "skipped", err: fmt.Errorf("no writer for tool")}
		}

		var err error
		if tool.ID == tools.ToolClaudeCode {
			err = tools.WriteClaudeCode(s.scope, s.telemetryOptIn)
		} else {
			err = tool.Write(s.scope)
		}
		if err != nil {
			return writeCompleteMsg{index: index, status: "failed", err: err}
		}
		return writeCompleteMsg{index: index, status: "done"}
	}
}

func (s *ConfigureStep) allComplete() bool {
	for _, status := range s.statuses {
		if status.writing && status.status == "" && status.err == nil {
			return false
		}
	}
	return true
}

func (s *ConfigureStep) hasErrors() bool {
	for _, status := range s.statuses {
		if status.err != nil {
			return true
		}
	}
	return false
}

func (s *ConfigureStep) View() string {
	var b strings.Builder

	b.WriteString(Styles.Title.Render("Configuring tools with Kimchi models"))
	b.WriteString("\n\n")

	spinChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinIdx := 0

	for _, status := range s.statuses {
		var icon string
		var msg string

		if status.err != nil {
			icon = Styles.Error.Render("✗")
			msg = Styles.Error.Render(fmt.Sprintf(" %s", status.err))
		} else if status.status == "done" {
			icon = Styles.Success.Render("✓")
			msg = Styles.Success.Render(" configured")
		} else if status.writing {
			spin := spinChars[spinIdx%len(spinChars)]
			spinIdx++
			icon = Styles.Spinner.Render(spin)
			msg = Styles.Spinner.Render(" writing...")
		} else {
			icon = "○"
			msg = " waiting"
		}

		// Add model information for each tool
		modelInfo := s.getModelInfoForTool(status.tool.ID)
		line := fmt.Sprintf("  %s %s%s", icon, status.tool.Name, msg)
		if modelInfo != "" {
			line += fmt.Sprintf("\n    %s", Styles.Desc.Render(modelInfo))
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	if s.done {
		b.WriteString("\n")
		if s.hasErrors() {
			b.WriteString(Styles.Warning.Render("Configuration completed with errors."))
			b.WriteString("\n")
			b.WriteString(Styles.Help.Render("Press enter to continue"))
		} else {
			b.WriteString(Styles.Success.Render("Configuration complete!"))
			b.WriteString("\n\n")
			b.WriteString("Your tools are now connected to Cast AI's inference endpoint:\n")
			b.WriteString(Styles.Success.Render("https://llm.cast.ai"))
			b.WriteString("\n\n")
			b.WriteString("Each tool has been configured with optimal models for its use case:")
			b.WriteString("\n")
			b.WriteString(Styles.Desc.Render("• Reasoning tasks → glm-5-fp8"))
			b.WriteString("\n")
			b.WriteString(Styles.Desc.Render("• Code generation → minimax-m2.5"))
			b.WriteString("\n")
			b.WriteString(Styles.Desc.Render("• Multi-modal tasks → kimi-k2.5"))
		}
	}

	return b.String()
}

func (s *ConfigureStep) getModelInfoForTool(toolID tools.ToolID) string {
	switch toolID {
	case tools.ToolClaudeCode:
		return "→ glm-5-fp8 (main) + minimax-m2.5 (subagents)"
	case tools.ToolOpenCode:
		return "→ glm-5-fp8 (reasoning) + minimax-m2.5 (coding) + kimi-k2.5 (vision)"
	case tools.ToolCursor, tools.ToolContinue:
		return "→ glm-5-fp8 (reasoning) + minimax-m2.5 (coding) + kimi-k2.5 (vision)"
	case tools.ToolWindsurf:
		return "→ minimax-m2.5 (coding) + glm-5-fp8 (reasoning) + kimi-k2.5 (vision)"
	case tools.ToolZed:
		return "→ minimax-m2.5 (primary coding model)"
	case tools.ToolCodex:
		return "→ minimax-m2.5 (code generation and debugging)"
	case tools.ToolCline:
		return "→ minimax-m2.5 (action) + glm-5-fp8 (planning)"
	case tools.ToolGSD2:
		return "→ glm-5-fp8 (default) + minimax-m2.5 (coding) + kimi-k2.5 (vision)"
	case tools.ToolGeneric:
		return "→ Environment variables for Cast AI endpoint"
	default:
		return ""
	}
}

func (s *ConfigureStep) Name() string {
	return "Configure"
}

func (s *ConfigureStep) Info() StepInfo {
	bindings := []KeyBinding{}
	if s.done && s.hasErrors() {
		bindings = append(bindings, BindingsConfirm)
	}
	return StepInfo{
		Name:        "Configure",
		KeyBindings: bindings,
	}
}
