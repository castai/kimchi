package tools

import (
	"fmt"
	"os"

	"github.com/castai/kimchi/internal/config"
)

const continueConfigPath = "~/.continue/config.json"

func init() {
	register(Tool{
		ID:          ToolContinue,
		Name:        "Continue",
		Description: "AI code assistant extension",
		ConfigPath:  continueConfigPath,
		BinaryName:  "continue",
		IsInstalled: detectContinue,
		Write:       writeContinue,
	})
}

func detectContinue() bool {
	globalPath, err := config.ScopePaths(config.ScopeGlobal, continueConfigPath)
	if err != nil {
		return false
	}
	if _, err := os.Stat(globalPath); err == nil {
		return true
	}
	return false
}

func writeContinue(scope config.ConfigScope, apiKey string, models ModelConfig) error {
	if apiKey == "" {
		return fmt.Errorf("API key not configured")
	}

	path, err := config.ScopePaths(scope, continueConfigPath)
	if err != nil {
		return fmt.Errorf("get config path: %w", err)
	}

	existing, err := config.ReadJSON(path)
	if err != nil {
		return fmt.Errorf("read existing config: %w", err)
	}

	existingModels, _ := existing["models"].([]any)
	hasMainModel := false
	hasCodingModel := false
	hasSubModel := false

	for _, m := range existingModels {
		modelMap, ok := m.(map[string]any)
		if !ok {
			continue
		}
		title, _ := modelMap["title"].(string)
		if title == models.Main.DisplayName {
			hasMainModel = true
		}
		if title == models.Coding.DisplayName {
			hasCodingModel = true
		}
		if title == models.Sub.DisplayName {
			hasSubModel = true
		}
	}

	if !hasMainModel {
		existingModels = append(existingModels, map[string]any{
			"title":         models.Main.DisplayName,
			"provider":      "openai",
			"model":         models.Main.Slug,
			"apiBase":       baseURL,
			"apiKey":        apiKey,
			"contextLength": models.Main.Limits.ContextWindow,
			"completionOptions": map[string]any{
				"maxTokens": models.Main.Limits.MaxOutputTokens,
			},
		})
	}

	if !hasCodingModel {
		existingModels = append(existingModels, map[string]any{
			"title":         models.Coding.DisplayName,
			"provider":      "openai",
			"model":         models.Coding.Slug,
			"apiBase":       baseURL,
			"apiKey":        apiKey,
			"contextLength": models.Coding.Limits.ContextWindow,
			"completionOptions": map[string]any{
				"maxTokens": models.Coding.Limits.MaxOutputTokens,
			},
		})
	}

	if !hasSubModel {
		existingModels = append(existingModels, map[string]any{
			"title":         models.Sub.DisplayName,
			"provider":      "openai",
			"model":         models.Sub.Slug,
			"apiBase":       baseURL,
			"apiKey":        apiKey,
			"contextLength": models.Sub.Limits.ContextWindow,
			"completionOptions": map[string]any{
				"maxTokens": models.Sub.Limits.MaxOutputTokens,
			},
		})
	}

	existing["models"] = existingModels

	if existing["tabAutocompleteModel"] == nil {
		existing["tabAutocompleteModel"] = map[string]any{
			"title":    models.Main.DisplayName,
			"provider": "openai",
			"model":    models.Main.Slug,
			"apiBase":  baseURL,
			"apiKey":   apiKey,
		}
	}

	if existing["embeddingsProvider"] == nil {
		existing["embeddingsProvider"] = map[string]any{
			"provider": "openai",
			"model":    "text-embedding-3-small",
			"apiBase":  baseURL,
			"apiKey":   apiKey,
		}
	}

	if err := config.WriteJSON(path, existing); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}
