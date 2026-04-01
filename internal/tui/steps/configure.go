package steps

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/castai/kimchi/internal/config"
	"github.com/castai/kimchi/internal/provider/claudecode"
	"github.com/castai/kimchi/internal/tools"
	tea "github.com/charmbracelet/bubbletea"
)

type ConfigureParams struct {
	ToolIDs        []tools.ToolID
	Mode           config.ConfigMode
	Scope          config.ConfigScope
	TelemetryOptIn bool
	APIKey         string
}

type ConfigureStep struct {
	toolIDs        []tools.ToolID
	mode           config.ConfigMode
	scope          config.ConfigScope
	telemetryOptIn bool
	apiKey         string
	saveErr        error
	warning        string
	done           bool
	startOnce      sync.Once
}

type saveCompleteMsg struct {
	err     error
	warning string
}

type startSaveMsg struct{}

func NewConfigureStep(params ConfigureParams) *ConfigureStep {
	return &ConfigureStep{
		toolIDs:        params.ToolIDs,
		mode:           params.Mode,
		scope:          params.Scope,
		telemetryOptIn: params.TelemetryOptIn,
		apiKey:         params.APIKey,
	}
}

func (s *ConfigureStep) Init() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return startSaveMsg{}
	})
}

func (s *ConfigureStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch m := msg.(type) {
	case startSaveMsg:
		var cmd tea.Cmd
		s.startOnce.Do(func() {
			cmd = s.savePreferences()
		})
		if cmd != nil {
			return s, cmd
		}
		return s, nil

	case saveCompleteMsg:
		s.saveErr = m.err
		s.warning = m.warning
		s.done = true
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

func (s *ConfigureStep) savePreferences() tea.Cmd {
	return func() tea.Msg {
		toolStrings := make([]string, len(s.toolIDs))
		for i, id := range s.toolIDs {
			toolStrings[i] = string(id)
		}

		if err := config.SavePreferences(s.apiKey, s.mode, toolStrings, string(s.scope), s.telemetryOptIn); err != nil {
			return saveCompleteMsg{err: err}
		}

		// In override mode, write directly to tool config files.
		if s.mode == config.ModeOverride {
			for _, toolID := range s.toolIDs {
				// Claude Code needs special handling for telemetry opt-in.
				if toolID == tools.ToolClaudeCode {
					if err := claudecode.WriteConfig(s.scope, s.telemetryOptIn); err != nil {
						return saveCompleteMsg{err: fmt.Errorf("configure Claude Code: %w", err)}
					}
					continue
				}

				tool, ok := tools.ByID(toolID)
				if !ok || tool.Write == nil {
					continue
				}
				if err := tool.Write(s.scope); err != nil {
					return saveCompleteMsg{err: fmt.Errorf("configure %s: %w", tool.Name, err)}
				}
			}

			var warning string
			if s.apiKey != "" {
				if _, err := config.ExportEnvToShellProfile(tools.APIKeyEnv, s.apiKey); err != nil {
					warning = fmt.Sprintf("Could not export %s to shell profile: %v", tools.APIKeyEnv, err)
				}
			}
			return saveCompleteMsg{warning: warning}
		}

		// In inject mode, IDE tools (non-wrappable) still need config written
		// directly because they cannot be launched via a kimchi wrapper.
		if s.mode == config.ModeInject {
			for _, toolID := range s.toolIDs {
				if toolID.IsWrappable() {
					continue
				}
				tool, ok := tools.ByID(toolID)
				if !ok || tool.Write == nil {
					continue
				}
				if err := tool.Write(s.scope); err != nil {
					return saveCompleteMsg{err: fmt.Errorf("configure %s: %w", tool.Name, err)}
				}
			}
		}

		return saveCompleteMsg{}
	}
}

func (s *ConfigureStep) View() string {
	var b strings.Builder

	if !s.done {
		b.WriteString(fmt.Sprintf("  %s Saving preferences...\n", "⠋"))
		return b.String()
	}

	if s.saveErr != nil {
		b.WriteString(Styles.Error.Render(fmt.Sprintf("✗ Failed to save preferences: %v", s.saveErr)))
		b.WriteString("\n\n")
		b.WriteString(Styles.Help.Render("Press enter to continue"))
		return b.String()
	}

	b.WriteString(Styles.Success.Render("✓ Configuration complete"))
	b.WriteString("\n\n")

	if s.mode == config.ModeInject {
		b.WriteString(Styles.Desc.Render("Wrote:"))
		b.WriteString(fmt.Sprintf("  %s\n\n", config.ConfigPath()))
		for _, toolID := range s.toolIDs {
			tool, ok := tools.ByID(toolID)
			if !ok {
				continue
			}
			if toolID.IsWrappable() {
				b.WriteString(fmt.Sprintf("  %s kimchi %s\n", Styles.Success.Render("→"), tool.BinaryName))
			} else {
				b.WriteString(fmt.Sprintf("  %s %s %s\n", Styles.Success.Render("→"), tool.Name, Styles.Desc.Render("(config written directly)")))
			}
		}
	} else {
		b.WriteString(Styles.Desc.Render("Wrote:"))
		b.WriteString(fmt.Sprintf("  %s\n", config.ConfigPath()))
		for _, toolID := range s.toolIDs {
			if tool, ok := tools.ByID(toolID); ok {
				b.WriteString(fmt.Sprintf("  %s\n", tool.ConfigPath))
			}
		}
		b.WriteString("\n")
		b.WriteString("Run your tools directly — they are already configured.\n")
	}

	if s.warning != "" {
		b.WriteString(Styles.Warning.Render(s.warning))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(Styles.Help.Render("Press enter to continue"))

	return b.String()
}

func (s *ConfigureStep) Name() string {
	return "Saving preferences"
}

func (s *ConfigureStep) Info() StepInfo {
	bindings := []KeyBinding{}
	if s.done {
		bindings = append(bindings, BindingsConfirm)
	}
	return StepInfo{
		Name:        "Saving preferences",
		KeyBindings: bindings,
	}
}
