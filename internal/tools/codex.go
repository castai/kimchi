package tools

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/castai/kimchi/internal/config"
)

const codexConfigPath = "~/.codex/config.toml"
const codexAgentsPath = "~/.codex/AGENTS.md"
const codexCatalogPath = "~/.codex/kimchi-models.json"
const envKeyInstructions = "Set the " + APIKeyEnv + " environment variable with your Cast AI API key. You can add it to your shell profile (~/.zshrc, ~/.bashrc) or a .env file."

func codexAgentMD() string {
	return `# Cast AI Configuration

This project uses Cast AI's open-source models:
- ` + MainModel.Slug + ` for reasoning, planning, and image processing (primary model)
- ` + CodingModel.Slug + ` for coding/execution (subagent)

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

type codexReasoningLevel struct {
	Effort      string `json:"effort"`
	Description string `json:"description"`
}

type codexTruncationPolicy struct {
	Mode  string `json:"mode"`
	Limit int    `json:"limit"`
}

type codexModelEntry struct {
	// Required fields
	Slug                       string                `json:"slug"`
	DisplayName                string                `json:"display_name"`
	ShellType                  string                `json:"shell_type"`
	Visibility                 string                `json:"visibility"`
	SupportedInAPI             bool                  `json:"supported_in_api"`
	Priority                   int                   `json:"priority"`
	BaseInstructions           string                `json:"base_instructions"`
	SupportsReasoningSummaries bool                  `json:"supports_reasoning_summaries"`
	SupportVerbosity           bool                  `json:"support_verbosity"`
	TruncationPolicy           codexTruncationPolicy `json:"truncation_policy"`
	SupportsParallelToolCalls  bool                  `json:"supports_parallel_tool_calls"`
	ExperimentalSupportedTools []string              `json:"experimental_supported_tools"`
	SupportedReasoningLevels   []codexReasoningLevel `json:"supported_reasoning_levels"`
	// Optional but useful
	Description           string   `json:"description,omitempty"`
	ContextWindow         int      `json:"context_window,omitempty"`
	InputModalities       []string `json:"input_modalities,omitempty"`
	ApplyPatchToolType    string   `json:"apply_patch_tool_type,omitempty"`
	DefaultReasoningLevel string   `json:"default_reasoning_level,omitempty"`
}

type codexCatalog struct {
	Models []codexModelEntry `json:"models"`
}

func writeModelCatalog(path string) error {
	var models []codexModelEntry
	for _, m := range allModels {
		entry := codexModelEntry{
			Slug:             m.Slug,
			DisplayName:      m.displayName,
			Description:      m.description,
			ShellType:        "shell_command",
			Visibility:       "list",
			SupportedInAPI:   true,
			BaseInstructions: "",
			// Deliberately higher than the Codex default of 10,000 tokens
			// (https://github.com/openai/codex/blob/main/codex-rs/core/models.json#L12)
			// because tool outputs in coding use cases (file reads, docs) can be large.
			// See: https://github.com/openai/codex/issues/6426
			TruncationPolicy:           codexTruncationPolicy{Mode: "tokens", Limit: 25_000},
			SupportsParallelToolCalls:  m.toolCall,
			ExperimentalSupportedTools: []string{},
			ContextWindow:              m.limits.contextWindow,
			InputModalities:            m.inputModalities,
			ApplyPatchToolType:         "function",
		}
		if m.reasoning {
			entry.DefaultReasoningLevel = "medium"
			entry.SupportedReasoningLevels = []codexReasoningLevel{
				{Effort: "low", Description: "Fast responses with lighter reasoning"},
				{Effort: "medium", Description: "Balances speed and reasoning depth"},
				{Effort: "high", Description: "Greater reasoning depth for complex problems"},
			}
		} else {
			entry.DefaultReasoningLevel = "none"
			entry.SupportedReasoningLevels = []codexReasoningLevel{
				{Effort: "none", Description: "No reasoning"},
			}
		}
		models = append(models, entry)
	}
	data, err := json.MarshalIndent(codexCatalog{Models: models}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal catalog: %w", err)
	}
	return config.WriteFile(path, data)
}

func writeCodex(scope config.ConfigScope) error {
	configPath, err := config.ScopePaths(scope, codexConfigPath)
	if err != nil {
		return fmt.Errorf("get config path: %w", err)
	}

	// Read existing config or start fresh
	cfg, err := config.ReadTOML(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	// GLM is the coding subagent used as the fixed default for Codex (a coding-focused tool)
	cfg["model"] = CodingModel.Slug
	cfg["model_provider"] = providerName
	cfg["suppress_unstable_features_warning"] = true

	// Add model provider
	providers, ok := cfg["model_providers"].(map[string]any)
	if !ok {
		if cfg["model_providers"] != nil {
			return fmt.Errorf("codex config file %s seems malformed, we don't want to destroy your changes: expected \"model_providers\" to be a TOML table, please fix the file and try again", configPath)
		}

		providers = make(map[string]any)
		cfg["model_providers"] = providers
	}

	providers["kimchi"] = map[string]any{
		"name":                 "Kimchi by Cast AI",
		"base_url":             baseURL,
		"env_key":              APIKeyEnv,
		"env_key_instructions": envKeyInstructions,
		"wire_api":             "responses",
	}

	// Write model catalog and reference it in config
	catalogPath, err := config.ScopePaths(scope, codexCatalogPath)
	if err != nil {
		return fmt.Errorf("get catalog path: %w", err)
	}
	if err := writeModelCatalog(catalogPath); err != nil {
		return fmt.Errorf("write model catalog: %w", err)
	}
	cfg["model_catalog_json"] = catalogPath

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
