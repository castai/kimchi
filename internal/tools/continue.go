package tools

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/castai/kimchi/internal/config"
)

func init() {
	register(Tool{
		ID:          ToolContinue,
		Name:        "Continue",
		Description: "AI code assistant extension",
		ConfigPath:  "~/.continue/config.json",
		BinaryName:  "continue",
		IsInstalled: detectContinue,
		Write:       writeContinue,
	})
}

func detectContinue() bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	configPath := filepath.Join(homeDir, ".continue", "config.json")
	if _, err := os.Stat(configPath); err == nil {
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

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	continueDir := filepath.Join(homeDir, ".continue")
	if err := os.MkdirAll(continueDir, 0755); err != nil {
		return fmt.Errorf("create continue directory: %w", err)
	}

	configPath := filepath.Join(continueDir, "config.json")

	existing, err := config.ReadJSON(configPath)
	if err != nil {
		return fmt.Errorf("read existing config: %w", err)
	}

	models, _ := existing["models"].([]any)
	hasReasoningModel := false
	hasCodingModel := false

	for _, m := range models {
		modelMap, ok := m.(map[string]any)
		if !ok {
			continue
		}
		title, _ := modelMap["title"].(string)
		if title == "GLM-5-FP8 (Cast AI)" {
			hasReasoningModel = true
		}
		if title == "MiniMax-M2.5 (Cast AI)" {
			hasCodingModel = true
		}
	}

	if !hasReasoningModel {
		models = append(models, map[string]any{
			"title":         "GLM-5-FP8 (Cast AI)",
			"provider":      "openai",
			"model":         reasoningModel,
			"apiBase":       baseURL,
			"apiKey":        apiKey,
			"contextLength": reasoningContext,
			"completionOptions": map[string]any{
				"maxTokens": reasoningOutput,
			},
		})
	}

	if !hasCodingModel {
		models = append(models, map[string]any{
			"title":         "MiniMax-M2.5 (Cast AI)",
			"provider":      "openai",
			"model":         codingModel,
			"apiBase":       baseURL,
			"apiKey":        apiKey,
			"contextLength": codingContext,
			"completionOptions": map[string]any{
				"maxTokens": codingOutput,
			},
		})
	}

	existing["models"] = models

	if existing["tabAutocompleteModel"] == nil {
		existing["tabAutocompleteModel"] = map[string]any{
			"title":    "MiniMax-M2.5 (Cast AI)",
			"provider": "openai",
			"model":    codingModel,
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

	if err := config.WriteJSON(configPath, existing); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}
