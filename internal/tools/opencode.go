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

func writeOpenCode(scope config.ConfigScope) error {
	apiKey, err := config.GetAPIKey()
	if err != nil {
		return fmt.Errorf("get API key: %w", err)
	}
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
	providers[ProviderName] = OpenCodeProviderConfig(apiKey)
	existing["provider"] = providers

	existing["compaction"] = map[string]any{
		"auto": true,
	}

	if err := config.WriteJSON(path, existing); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// OpenCodeProviderConfig returns the provider configuration map for OpenCode.
func OpenCodeProviderConfig(apiKey string) map[string]any {
	return map[string]any{
		"npm":  "@ai-sdk/openai-compatible",
		"name": "Kimchi by Cast AI",
		"options": map[string]any{
			"baseURL":      BaseURL,
			"litellmProxy": true,
			"apiKey":       apiKey,
		},
		"models": map[string]any{
			ReasoningModel.Slug: map[string]any{
				"name":      ReasoningModel.Slug,
				"tool_call": ReasoningModel.ToolCall,
				"reasoning": ReasoningModel.Reasoning,
				"limit": map[string]any{
					"context": ReasoningModel.Limits.ContextWindow,
					"output":  ReasoningModel.Limits.MaxOutputTokens,
				},
			},
			CodingModel.Slug: map[string]any{
				"name":      CodingModel.Slug,
				"tool_call": CodingModel.ToolCall,
				"reasoning": CodingModel.Reasoning,
				"limit": map[string]any{
					"context": CodingModel.Limits.ContextWindow,
					"output":  CodingModel.Limits.MaxOutputTokens,
				},
			},
			ImageModel.Slug: map[string]any{
				"name":      ImageModel.Slug,
				"tool_call": ImageModel.ToolCall,
				"reasoning": ImageModel.Reasoning,
				"limit": map[string]any{
					"context": ImageModel.Limits.ContextWindow,
					"output":  ImageModel.Limits.MaxOutputTokens,
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
