package tools

import (
	"fmt"
	"os/exec"

	"github.com/castai/kimchi/internal/config"
)

func init() {
	register(Tool{
		ID:          ToolOpenCode,
		Name:        "OpenCode",
		Description: "Agentic coding CLI",
		ConfigPath:  "~/.config/opencode/opencode.json",
		BinaryName:  "opencode",
		IsInstalled: detectBinary("opencode"),
		Write:       writeOpenCode,
	})
}

func writeOpenCode(scope config.ConfigScope, apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("API key not configured")
	}

	path, err := config.ScopePaths(scope, "~/.config/opencode/opencode.json")
	if err != nil {
		return fmt.Errorf("get config path: %w", err)
	}

	existing, err := config.ReadJSON(path)
	if err != nil {
		return fmt.Errorf("read existing config: %w", err)
	}

	existing["$schema"] = "https://opencode.ai/config.json"

	providers, _ := existing["provider"].(map[string]any)
	if providers == nil {
		providers = make(map[string]any)
	}
	providers[providerName] = OpenCodeProviderConfig(apiKey)
	existing["provider"] = providers

	existing["model"] = providerName + "/" + MainModel.Slug

	if _, ok := existing["compaction"]; !ok {
		existing["compaction"] = map[string]any{
			"auto": true,
		}
	}

	// Add or update Kimchi plugin while preserving existing plugins
	// Plugin format is an array of arrays: [[npm_package, config], ...]
	telemetryEnabled, _ := config.IsTelemetryEnabled()

	pluginConfig := map[string]any{}
	if telemetryEnabled {
		pluginConfig["telemetry"] = true
		pluginConfig["logsEndpoint"] = "https://api.cast.ai/ai-optimizer/v1beta/logs:ingest"
		pluginConfig["metricsEndpoint"] = "https://api.cast.ai/ai-optimizer/v1beta/metrics:ingest"
	}

	// Get existing plugins or create new array
	var plugins []any
	if existingPlugins, ok := existing["plugin"].([]any); ok {
		plugins = existingPlugins
	}

	// Find and update existing Kimchi plugin or append new one
	kimchiPlugin := []any{"@kimchi-dev/opencode-kimchi", pluginConfig}
	found := false
	for i, p := range plugins {
		if pluginArr, ok := p.([]any); ok && len(pluginArr) > 0 {
			if pkgName, ok := pluginArr[0].(string); ok && pkgName == "@kimchi-dev/opencode-kimchi" {
				plugins[i] = kimchiPlugin
				found = true
				break
			}
		}
	}
	if !found {
		plugins = append(plugins, kimchiPlugin)
	}
	existing["plugin"] = plugins

	if err := config.WriteJSON(path, existing); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// OpenCodeProviderConfig returns the provider configuration map for OpenCode.
func OpenCodeProviderConfig(apiKey string) map[string]any {
	return map[string]any{
		"npm":  "@ai-sdk/openai-compatible",
		"name": "Kimchi",
		"options": map[string]any{
			"baseURL":      baseURL,
			"litellmProxy": true,
			"apiKey":       apiKey,
		},
		"models": map[string]any{
			MainModel.Slug: map[string]any{
				"name":      MainModel.Slug,
				"tool_call": MainModel.toolCall,
				"reasoning": MainModel.reasoning,
				"limit": map[string]any{
					"context": MainModel.limits.contextWindow,
					"output":  MainModel.limits.maxOutputTokens,
				},
			},
			CodingModel.Slug: map[string]any{
				"name":      CodingModel.Slug,
				"tool_call": CodingModel.toolCall,
				"reasoning": CodingModel.reasoning,
				"limit": map[string]any{
					"context": CodingModel.limits.contextWindow,
					"output":  CodingModel.limits.maxOutputTokens,
				},
			},
			SubModel.Slug: map[string]any{
				"name":      SubModel.Slug,
				"tool_call": SubModel.toolCall,
				"reasoning": SubModel.reasoning,
				"limit": map[string]any{
					"context": SubModel.limits.contextWindow,
					"output":  SubModel.limits.maxOutputTokens,
				},
			},
		},
	}
}

func detectBinary(name string) func() bool {
	return func() bool {
		_, err := exec.LookPath(name)
		return err == nil
	}
}
