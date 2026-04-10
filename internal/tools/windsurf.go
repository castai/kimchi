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
	globalPath, err := config.ScopePaths(config.ScopeGlobal, getWindsurfConfigPath())
	if err == nil {
		appSupportPath := filepath.Dir(filepath.Dir(filepath.Dir(globalPath))) // trim /User/globalStorage/storage.json
		if _, err := os.Stat(appSupportPath); err == nil {
			return true
		}
	}

	if _, err := exec.LookPath("windsurf"); err == nil {
		return true
	}
	return false
}

func writeWindsurf(scope config.ConfigScope, apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("API key not configured")
	}

	storagePath, err := config.ScopePaths(scope, getWindsurfConfigPath())
	if err != nil {
		return fmt.Errorf("get config path: %w", err)
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

	kimiConfig := map[string]any{
		"apiProvider": "openai-native",
		"apiKey":      apiKey,
		"baseUrl":     baseURL,
		"modelId":     MainModel.Slug,
		"modelInfo": map[string]any{
			"maxTokens":           MainModel.limits.maxOutputTokens,
			"contextWindow":       MainModel.limits.contextWindow,
			"supportsImages":      MainModel.supportsImages,
			"supportsPromptCache": false,
		},
	}

	codingConfig := map[string]any{
		"apiProvider": "openai-native",
		"apiKey":      apiKey,
		"baseUrl":     baseURL,
		"modelId":     CodingModel.Slug,
		"modelInfo": map[string]any{
			"maxTokens":           CodingModel.limits.maxOutputTokens,
			"contextWindow":       CodingModel.limits.contextWindow,
			"supportsImages":      CodingModel.supportsImages,
			"supportsPromptCache": false,
		},
	}

	subConfig := map[string]any{
		"apiProvider": "openai-native",
		"apiKey":      apiKey,
		"baseUrl":     baseURL,
		"modelId":     SubModel.Slug,
		"modelInfo": map[string]any{
			"maxTokens":           SubModel.limits.maxOutputTokens,
			"contextWindow":       SubModel.limits.contextWindow,
			"supportsImages":      SubModel.supportsImages,
			"supportsPromptCache": false,
		},
	}

	apiConfigs["castai-kimi"] = kimiConfig
	apiConfigs["castai-coding"] = codingConfig
	apiConfigs["castai-sub"] = subConfig
	state["apiConfigs"] = apiConfigs
	state["currentApiConfigName"] = "castai-kimi"
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
