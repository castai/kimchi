package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/castai/kimchi/internal/config"
)

func init() {
	register(Tool{
		ID:          ToolCursor,
		Name:        "Cursor",
		Description: "AI-powered code editor",
		ConfigPath:  "~/.cursor/config.json",
		BinaryName:  "cursor",
		IsInstalled: detectCursor,
		Write:       writeCursor,
	})
}

func detectCursor() bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	configPath := filepath.Join(homeDir, ".cursor", "config.json")
	if _, err := os.Stat(configPath); err == nil {
		return true
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

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	cursorDir := filepath.Join(homeDir, ".cursor")
	if err := os.MkdirAll(cursorDir, 0755); err != nil {
		return fmt.Errorf("create cursor directory: %w", err)
	}

	configPath := filepath.Join(cursorDir, "config.json")

	existing, err := config.ReadJSON(configPath)
	if err != nil {
		return fmt.Errorf("read existing config: %w", err)
	}

	openAIConfig, _ := existing["openAICompatible"].(map[string]any)
	if openAIConfig == nil {
		openAIConfig = make(map[string]any)
	}

	openAIConfig["apiKey"] = apiKey
	openAIConfig["baseURL"] = baseURL
	existing["openAICompatible"] = openAIConfig

	if err := config.WriteJSON(configPath, existing); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}
