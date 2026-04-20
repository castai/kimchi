package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/castai/kimchi/internal/config"
)

const claudeCodeConfigPath = "~/.claude/settings.json"

func init() {
	register(Tool{
		ID:          ToolClaudeCode,
		Name:        "Claude Code",
		Description: "Anthropic's Claude Code CLI",
		ConfigPath:  claudeCodeConfigPath,
		BinaryName:  "claude",
		IsInstalled: detectBinary("claude"),
		Write:       writeClaudeCode,
	})
}

// writeClaudeCode configures Claude Code to use the Kimchi proxy.
// It only configures the base URL and API key - model selection is handled by Claude Code internally.
func writeClaudeCode(scope config.ConfigScope, apiKey string) error {
	configPath, err := config.ScopePaths(scope, claudeCodeConfigPath)
	if err != nil {
		return fmt.Errorf("get claude config path: %w", err)
	}

	// Ensure directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create claude config directory: %w", err)
	}

	// Read existing config or create new
	var existing map[string]any
	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, &existing); err != nil {
			// If unmarshal fails, start fresh but preserve file for debugging
			existing = make(map[string]any)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read claude config: %w", err)
	} else {
		existing = make(map[string]any)
	}

	// Get or create env block
	env, ok := existing["env"].(map[string]any)
	if !ok {
		env = make(map[string]any)
	}

	// Configure Kimchi proxy URLs only (no model selection - CC handles that internally)
	env["ANTHROPIC_BASE_URL"] = anthropicBaseURL
	env["ANTHROPIC_API_KEY"] = apiKey
	// Disable beta headers if proxy doesn't support them
	env["CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS"] = "1"

	existing["env"] = env

	// Write back with proper formatting
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal claude config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("write claude config: %w", err)
	}

	return nil
}
