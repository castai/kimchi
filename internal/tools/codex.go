package tools

import (
	"fmt"
	"os"

	"github.com/castai/kimchi/internal/config"
)

const codexConfigPath = "~/.codex/config.toml"
const codexAgentsPath = "~/.codex/AGENTS.md"
const envKeyInstructions = "Set the " + APIKeyEnv + " environment variable with your Cast AI API key. You can add it to your shell profile (~/.zshrc, ~/.bashrc) or a .env file."

func codexAgentMD() string {
	return `# Cast AI Configuration

This project uses Cast AI's open-source models:
- ` + reasoningModel + ` for reasoning/planning
- ` + codingModel + ` for coding/execution

Set the ` + APIKeyEnv + ` environment variable with your Cast AI API key.
`
}

func init() {
	register(Tool{
		ID:          ToolCodex,
		Name:        "Codex",
		Description: "OpenAI coding CLI",
		ConfigPath:  codexConfigPath,
		BinaryName:  "codex",
		IsInstalled: detectBinary("codex"),
		Write:       writeCodex,
	})
}

func writeCodex(scope config.ConfigScope) error {
	return WriteCodex(scope, false)
}

func WriteCodex(scope config.ConfigScope, makeDefault bool) error {
	configPath, err := config.ScopePaths(scope, codexConfigPath)
	if err != nil {
		return fmt.Errorf("get config path: %w", err)
	}

	// Read existing config or start fresh
	cfg, err := config.ReadTOML(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	// Set top-level config only if user opted in
	if makeDefault {
		cfg["model"] = codingModel
		cfg["model_provider"] = providerName
	}

	// Add model provider
	if cfg["model_providers"] == nil {
		cfg["model_providers"] = make(map[string]any)
	}
	providers := cfg["model_providers"].(map[string]any)
	providers["kimchi"] = map[string]any{
		"name":                 "Kimchi by Cast AI",
		"base_url":             baseURL,
		"env_key":              APIKeyEnv,
		"env_key_instructions": envKeyInstructions,
		"wire_api":             "responses",
	}

	if err := config.WriteTOML(configPath, cfg); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	// Write AGENTS.md only if it doesn't exist
	instructionsPath, err := config.ScopePaths(scope, codexAgentsPath)
	if err != nil {
		return fmt.Errorf("get AGENTS.md path: %w", err)
	}

	if _, err := os.Stat(instructionsPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("stat AGENTS.md: %w", err)
		}
		if err := config.WriteFile(instructionsPath, []byte(codexAgentMD())); err != nil {
			return fmt.Errorf("write AGENTS.md: %w", err)
		}
	}

	return nil
}
