package tools

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/castai/kimchi/internal/config"
)

const cursorConfigPath = "~/.cursor/config.json"

func init() {
	register(Tool{
		ID:          ToolCursor,
		Name:        "Cursor",
		Description: "AI-powered code editor",
		ConfigPath:  cursorConfigPath,
		BinaryName:  "cursor",
		IsInstalled: detectCursor,
		Write:       writeCursor,
	})
}

func detectCursor() bool {
	globalPath, err := config.ScopePaths(config.ScopeGlobal, cursorConfigPath)
	if err == nil {
		if _, err := os.Stat(globalPath); err == nil {
			return true
		}
	}
	if _, err := exec.LookPath("cursor"); err == nil {
		return true
	}
	return false
}

func writeCursor(scope config.ConfigScope) error {
	apiKey, err := config.GetAPIKey()
	if err != nil {
		return fmt.Errorf("get API key: %w", err)
	}
	if apiKey == "" {
		return fmt.Errorf("API key not configured")
	}

	path, err := config.ScopePaths(scope, cursorConfigPath)
	if err != nil {
		return fmt.Errorf("get config path: %w", err)
	}

	existing, err := config.ReadJSON(path)
	if err != nil {
		return fmt.Errorf("read existing config: %w", err)
	}

	openAIConfig, _ := existing["openAICompatible"].(map[string]any)
	if openAIConfig == nil {
		openAIConfig = make(map[string]any)
	}

	openAIConfig["apiKey"] = apiKey
	openAIConfig["baseURL"] = BaseURL
	existing["openAICompatible"] = openAIConfig

	if err := config.WriteJSON(path, existing); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}
