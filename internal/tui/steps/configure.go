package steps

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/castai/kimchi/internal/config"
	"github.com/castai/kimchi/internal/tools"
	tea "github.com/charmbracelet/bubbletea"
)

type ConfigureParams struct {
	ToolIDs        []tools.ToolID
	Scope          config.ConfigScope
	TelemetryOptIn bool
	APIKey         string
}

type toolStatus struct {
	tool    tools.Tool
	status  string
	err     error
	writing bool
}

type ConfigureStep struct {
	toolIDs          []tools.ToolID
	scope            config.ConfigScope
	telemetryOptIn   bool
	apiKey           string
	shellProfilePath string
	shellProfileErr  error
	statuses         []toolStatus
	done             bool
	startOnce        sync.Once
}

type writeCompleteMsg struct {
	index  int
	status string
	err    error
}

type startWriteMsg struct{}

func NewConfigureStep(params ConfigureParams) *ConfigureStep {
	return &ConfigureStep{
		toolIDs:        params.ToolIDs,
		scope:          params.Scope,
		telemetryOptIn: params.TelemetryOptIn,
		apiKey:         params.APIKey,
		statuses:       make([]toolStatus, len(params.ToolIDs)),
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
		}
		return s, nil

	case writeCompleteMsg:
		s.statuses[m.index] = toolStatus{
			tool:   s.statuses[m.index].tool,
			status: m.status,
			err:    m.err,
		}
		if s.allComplete() {
			s.exportAPIKeyToShellProfile()
			s.done = true
		}
		return s, nil

	case tea.KeyMsg:
		if s.done {
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

func (s *ConfigureStep) exportAPIKeyToShellProfile() {
	if s.apiKey == "" {
		return
	}
	path, err := config.ExportEnvToShellProfile(tools.APIKeyEnv, s.apiKey)
	s.shellProfilePath = path
	s.shellProfileErr = err
}

func (s *ConfigureStep) ShellProfilePath() string {
	return s.shellProfilePath
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
			b.WriteString(fmt.Sprintf("• Reasoning tasks → %s", tools.ReasoningModel.Slug))
			b.WriteString("\n")
			b.WriteString(fmt.Sprintf("• Code generation → %s", tools.CodingModel.Slug))
			b.WriteString("\n")
			b.WriteString(fmt.Sprintf("• Multi-modal tasks → %s", tools.ImageModel.Slug))
			b.WriteString("\n")
			if s.shellProfilePath != "" {
				b.WriteString(fmt.Sprintf("\n%s exported to %s\n", tools.APIKeyEnv, s.shellProfilePath))
			} else if s.shellProfileErr != nil {
				b.WriteString(fmt.Sprintf("\n%s\n", Styles.Warning.Render(fmt.Sprintf("Could not export %s to shell profile: %v", tools.APIKeyEnv, s.shellProfileErr))))
			}
			b.WriteString("\n")
			b.WriteString(Styles.Help.Render("Press enter to continue"))
		}
	}

	return b.String()
}

func (s *ConfigureStep) getModelInfoForTool(toolID tools.ToolID) string {
	r := tools.ReasoningModel.Slug
	c := tools.CodingModel.Slug
	i := tools.ImageModel.Slug

	switch toolID {
	case tools.ToolClaudeCode:
		return fmt.Sprintf("→ %s (plan mode) + %s (execution mode)", r, c)
	case tools.ToolOpenCode:
		return fmt.Sprintf("→ %s (reasoning) + %s (coding) + %s (vision)", r, c, i)
	case tools.ToolCursor, tools.ToolContinue:
		return fmt.Sprintf("→ %s (reasoning) + %s (coding) + %s (vision)", r, c, i)
	case tools.ToolWindsurf:
		return fmt.Sprintf("→ %s (coding) + %s (reasoning) + %s (vision)", c, r, i)
	case tools.ToolZed:
		return fmt.Sprintf("→ %s (primary coding model)", c)
	case tools.ToolCodex:
		return fmt.Sprintf("→ %s (code generation and debugging)", c)
	case tools.ToolCline:
		return fmt.Sprintf("→ %s (action) + %s (planning)", c, r)
	case tools.ToolGSD2:
		return fmt.Sprintf("→ %s (default) + %s (coding) + %s (vision)", r, c, i)
	case tools.ToolGeneric:
		return "→ Environment variables for Cast AI endpoint"
	default:
		return ""
	}
}

func (s *ConfigureStep) Name() string {
	return "Configuring tools"
}

func (s *ConfigureStep) Info() StepInfo {
	bindings := []KeyBinding{}
	if s.done {
		bindings = append(bindings, BindingsConfirm)
	}
	return StepInfo{
		Name:        "Configuring tools",
		KeyBindings: bindings,
	}
}
