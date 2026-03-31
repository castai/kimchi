package tools

import (
	"fmt"

	"github.com/castai/kimchi/internal/config"
)

func init() {
	register(Tool{
		ID:          ToolCline,
		Name:        "Cline",
		Description: "Autonomous coding agent",
		ConfigPath:  "~/.cline/data/globalState.json",
		BinaryName:  "cline",
		IsInstalled: detectBinary("cline"),
		Write:       writeCline,
	})
}

func writeCline(scope config.ConfigScope) error {
	apiKey, err := config.GetAPIKey()
	if err != nil {
		return fmt.Errorf("get API key: %w", err)
	}
	if apiKey == "" {
		return fmt.Errorf("API key not configured")
	}

	path, err := config.ScopePaths(scope, "~/.cline/data/globalState.json")
	if err != nil {
		return fmt.Errorf("get config path: %w", err)
	}

	existing, err := config.ReadJSON(path)
	if err != nil {
		return fmt.Errorf("read existing config: %w", err)
	}

	existing["ollamaBaseUrl"] = BaseURL
	existing["actModeApiProvider"] = "ollama"
	existing["actModeOllamaModelId"] = CodingModel.Slug
	existing["actModeOllamaBaseUrl"] = BaseURL
	existing["planModeApiProvider"] = "ollama"
	existing["planModeOllamaModelId"] = ReasoningModel.Slug
	existing["planModeOllamaBaseUrl"] = BaseURL
	existing["welcomeViewCompleted"] = true

	if err := config.WriteJSON(path, existing); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}
