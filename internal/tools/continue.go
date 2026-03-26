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

func writeContinue(scope config.ConfigScope) error {
	apiKey, err := config.GetAPIKey()
	if err != nil {
		return fmt.Errorf("get API key: %w", err)
	}
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

	models, _ := existing["models"].([]any)
	hasReasoningModel := false
	hasCodingModel := false
	hasImageModel := false

	for _, m := range models {
		modelMap, ok := m.(map[string]any)
		if !ok {
			continue
		}
		title, _ := modelMap["title"].(string)
		if title == reasoningModel.displayName {
			hasReasoningModel = true
		}
		if title == codingModel.displayName {
			hasCodingModel = true
		}
		if title == imageModel.displayName {
			hasImageModel = true
		}
	}

	if !hasReasoningModel {
		models = append(models, map[string]any{
			"title":         reasoningModel.displayName,
			"provider":      "openai",
			"model":         reasoningModel.slug,
			"apiBase":       baseURL,
			"apiKey":        apiKey,
			"contextLength": reasoningModel.limits.contextWindow,
			"completionOptions": map[string]any{
				"maxTokens": reasoningModel.limits.maxOutputTokens,
			},
		})
	}

	if !hasCodingModel {
		models = append(models, map[string]any{
			"title":         codingModel.displayName,
			"provider":      "openai",
			"model":         codingModel.slug,
			"apiBase":       baseURL,
			"apiKey":        apiKey,
			"contextLength": codingModel.limits.contextWindow,
			"completionOptions": map[string]any{
				"maxTokens": codingModel.limits.maxOutputTokens,
			},
		})
	}

	if !hasImageModel {
		models = append(models, map[string]any{
			"title":         imageModel.displayName,
			"provider":      "openai",
			"model":         imageModel.slug,
			"apiBase":       baseURL,
			"apiKey":        apiKey,
			"contextLength": imageModel.limits.contextWindow,
			"completionOptions": map[string]any{
				"maxTokens": imageModel.limits.maxOutputTokens,
			},
		})
	}

	existing["models"] = models

	if existing["tabAutocompleteModel"] == nil {
		existing["tabAutocompleteModel"] = map[string]any{
			"title":    codingModel.displayName,
			"provider": "openai",
			"model":    codingModel.slug,
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
