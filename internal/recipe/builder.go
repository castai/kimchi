package recipe

import (
	"strings"
	"time"
)

// ExportOptions carries the user's choices from the TUI wizard.
type ExportOptions struct {
	Name        string
	Author      string
	Description string
	Tags        []string
	UseCase     string

	IncludeAgentsMD        bool
	IncludeSkills          bool
	IncludeCustomCommands  bool
	IncludeAgents          bool
	IncludeTUI             bool
	IncludeThemeFiles      bool
	IncludePluginFiles     bool
	IncludeToolFiles       bool
}

// Build assembles a Recipe from OpenCode assets and the user's export options.
// Secrets in provider and MCP configs are replaced with placeholder strings.
func Build(assets *OpenCodeAssets, opts ExportOptions) (*Recipe, error) {
	cfg := assets.Config

	// Use the model from config as-is (e.g. "kimchi/kimi-k2.5" or "openai/gpt-4o").
	model := strField(cfg, "model")

	// Strip provider prefix for the recipe's top-level model field (human-readable slug).
	displaySlug := model
	if parts := strings.SplitN(model, "/", 2); len(parts) == 2 {
		displaySlug = parts[1]
	}

	ocCfg := &OpenCodeConfig{
		// Provider / model
		Providers:         mapField(cfg, "provider"),
		Model:             model,
		SmallModel:        strField(cfg, "small_model"),
		DefaultAgent:      strField(cfg, "default_agent"),
		DisabledProviders: strSliceField(cfg, "disabled_providers"),
		EnabledProviders:  strSliceField(cfg, "enabled_providers"),
		Plugin:            strSliceField(cfg, "plugin"),
		Snapshot:          boolPtrField(cfg, "snapshot"),

		// Portable instruction URLs (local paths/globs are machine-specific and excluded)
		Instructions: filterURLInstructions(cfg),

		// Behavior
		Compaction:     mapField(cfg, "compaction"),
		AgentConfigs:   mapField(cfg, "agent"),
		MCP:            mapField(cfg, "mcp"),
		Permission:     cfg["permission"],
		Tools:          mapField(cfg, "tools"),
		Experimental:   mapField(cfg, "experimental"),
		Formatter:      cfg["formatter"],
		LSP:            cfg["lsp"],
		InlineCommands: mapField(cfg, "command"),
	}

	if opts.IncludeAgentsMD {
		ocCfg.AgentsMD = assets.AgentsMD
	}
	if opts.IncludeSkills {
		ocCfg.Skills = assets.Skills
	}
	if opts.IncludeCustomCommands {
		ocCfg.CustomCommands = assets.CustomCommands
	}
	if opts.IncludeAgents {
		ocCfg.Agents = assets.Agents
	}
	if opts.IncludeTUI {
		ocCfg.TUI = assets.TUI
	}
	if opts.IncludeThemeFiles {
		ocCfg.ThemeFiles = assets.ThemeFiles
	}
	if opts.IncludePluginFiles {
		ocCfg.PluginFiles = assets.PluginFiles
	}
	if opts.IncludeToolFiles {
		ocCfg.ToolFiles = assets.ToolFiles
	}
	// Include files that are @-referenced from any selected markdown content.
	if opts.IncludeAgentsMD || opts.IncludeSkills || opts.IncludeCustomCommands || opts.IncludeAgents {
		ocCfg.ReferencedFiles = assets.ReferencedFiles
	}

	now := time.Now().UTC().Format(time.RFC3339)
	r := &Recipe{
		Name:        opts.Name,
		Version:     "0.1.0",
		Author:      opts.Author,
		Description: opts.Description,
		Tags:        opts.Tags,
		CreatedAt:   now,
		UpdatedAt:   now,
		Model:       displaySlug,
		UseCase:     opts.UseCase,
		Tools: ToolsMap{
			OpenCode: ocCfg,
		},
	}

	return r, nil
}

// mapField extracts a map[string]any from cfg[key], returning nil if absent or wrong type.
func mapField(cfg map[string]any, key string) map[string]any {
	v, ok := cfg[key].(map[string]any)
	if !ok {
		return nil
	}
	return v
}

// strField extracts a string from cfg[key], returning "" if absent or wrong type.
func strField(cfg map[string]any, key string) string {
	v, _ := cfg[key].(string)
	return v
}

// strSliceField extracts a []string from cfg[key], returning nil if absent or wrong type.
func strSliceField(cfg map[string]any, key string) []string {
	raw, ok := cfg[key].([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// boolPtrField extracts a *bool from cfg[key], returning nil if absent or wrong type.
func boolPtrField(cfg map[string]any, key string) *bool {
	v, ok := cfg[key].(bool)
	if !ok {
		return nil
	}
	return &v
}
