package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/castai/kimchi/internal/config"
)

func init() {
	register(Tool{
		ID:          ToolWindsurf,
		Name:        "Windsurf",
		Description: "AI-powered IDE with Roo Code",
		ConfigPath:  getWindsurfConfigPath(),
		BinaryName:  "windsurf",
		IsInstalled: detectWindsurf,
		Write:       writeWindsurf,
	})
}

func getWindsurfConfigPath() string {
	if runtime.GOOS == "darwin" {
		return "~/Library/Application Support/Windsurf/User/globalStorage/storage.json"
	}
	return "~/.config/windsurf/User/globalStorage/storage.json"
}

func detectWindsurf() bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	var appSupportPath string
	if runtime.GOOS == "darwin" {
		appSupportPath = filepath.Join(homeDir, "Library", "Application Support", "Windsurf")
	} else {
		appSupportPath = filepath.Join(homeDir, ".config", "windsurf")
	}

	if _, err := os.Stat(appSupportPath); err == nil {
		return true
	}

	if _, err := exec.LookPath("windsurf"); err == nil {
		return true
	}
	return false
}

func writeWindsurf(scope config.ConfigScope) error {
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

	var storagePath string
	if runtime.GOOS == "darwin" {
		storagePath = filepath.Join(homeDir, "Library", "Application Support", "Windsurf", "User", "globalStorage", "storage.json")
	} else {
		storagePath = filepath.Join(homeDir, ".config", "windsurf", "User", "globalStorage", "storage.json")
	}

	storageDir := filepath.Dir(storagePath)
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return fmt.Errorf("create storage directory: %w", err)
	}

	data, err := os.ReadFile(storagePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read storage: %w", err)
	}

	var storage map[string]any
	if len(data) > 0 {
		if err := json.Unmarshal(data, &storage); err != nil {
			storage = make(map[string]any)
		}
	} else {
		storage = make(map[string]any)
	}

	stateKey := "state.roo-cline"
	state, _ := storage[stateKey].(map[string]any)
	if state == nil {
		state = make(map[string]any)
	}

	apiConfigs, _ := state["apiConfigs"].(map[string]any)
	if apiConfigs == nil {
		apiConfigs = make(map[string]any)
	}

	castaiConfig := map[string]any{
		"apiProvider": "openai-native",
		"apiKey":      apiKey,
		"baseUrl":     baseURL,
		"modelId":     codingModel,
		"modelInfo": map[string]any{
			"maxTokens":           codingOutput,
			"contextWindow":       codingContext,
			"supportsImages":      false,
			"supportsPromptCache": false,
		},
	}

	reasoningConfig := map[string]any{
		"apiProvider": "openai-native",
		"apiKey":      apiKey,
		"baseUrl":     baseURL,
		"modelId":     reasoningModel,
		"modelInfo": map[string]any{
			"maxTokens":           reasoningOutput,
			"contextWindow":       reasoningContext,
			"supportsImages":      false,
			"supportsPromptCache": false,
		},
	}

	apiConfigs["castai-coding"] = castaiConfig
	apiConfigs["castai-reasoning"] = reasoningConfig
	state["apiConfigs"] = apiConfigs
	state["currentApiConfigName"] = "castai-coding"
	storage[stateKey] = state

	newData, err := json.MarshalIndent(storage, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal storage: %w", err)
	}

	if err := os.WriteFile(storagePath, newData, 0644); err != nil {
		return fmt.Errorf("write storage: %w", err)
	}

	return nil
}
