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

	providerConfig := map[string]any{
		"npm":  "@ai-sdk/openai-compatible",
		"name": "Kimchi by Cast AI",
		"options": map[string]any{
			"baseURL":      baseURL,
			"litellmProxy": true,
			"apiKey":       apiKey,
		},
		"models": map[string]any{
			reasoningModel.slug: map[string]any{
				"name":      reasoningModel.slug,
				"tool_call": reasoningModel.toolCall,
				"reasoning": reasoningModel.reasoning,
				"limit": map[string]any{
					"context": reasoningModel.limits.contextWindow,
					"output":  reasoningModel.limits.maxOutputTokens,
				},
			},
			codingModel.slug: map[string]any{
				"name":      codingModel.slug,
				"tool_call": codingModel.toolCall,
				"reasoning": codingModel.reasoning,
				"limit": map[string]any{
					"context": codingModel.limits.contextWindow,
					"output":  codingModel.limits.maxOutputTokens,
				},
			},
			imageModel.slug: map[string]any{
				"name":      imageModel.slug,
				"tool_call": imageModel.toolCall,
				"reasoning": imageModel.reasoning,
				"limit": map[string]any{
					"context": imageModel.limits.contextWindow,
					"output":  imageModel.limits.maxOutputTokens,
				},
			},
		},
	}

	providers, _ := existing["provider"].(map[string]any)
	if providers == nil {
		providers = make(map[string]any)
	}
	providers[providerName] = providerConfig
	existing["provider"] = providers

	existing["compaction"] = map[string]any{
		"auto": true,
	}

	if err := config.WriteJSON(path, existing); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

func detectBinary(name string) func() bool {
	return func() bool {
		_, err := exec.LookPath(name)
		return err == nil
	}
}
