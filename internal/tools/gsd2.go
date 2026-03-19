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
		ID:          ToolGSD2,
		Name:        "GSD2",
		Description: "Autonomous coding agent (Get Shit Done v2)",
		ConfigPath:  "~/.gsd/preferences.md",
		BinaryName:  "gsd",
		IsInstalled: detectGSD2,
		Write:       writeGSD2,
	})
}

func detectGSD2() bool {
	_, err := exec.LookPath("gsd")
	if err == nil {
		return true
	}
	_, err = exec.LookPath("gsd-cli")
	return err == nil
}

func writeGSD2(scope config.ConfigScope) error {
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

	gsdDir := filepath.Join(homeDir, ".gsd")
	agentDir := filepath.Join(gsdDir, "agent")

	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return fmt.Errorf("create GSD directories: %w", err)
	}

	modelsPath := filepath.Join(agentDir, "models.json")
	modelsContent := map[string]any{
		"providers": map[string]any{
			"castai": map[string]any{
				"name":         "Cast AI",
				"baseUrl":      baseURL,
				"apiKey":       apiKey,
				"api":          "openai",
				"defaultModel": reasoningModel,
				"models": []map[string]any{
					{
						"id":            reasoningModel,
						"name":          "GLM-5 FP8",
						"contextWindow": reasoningContext,
						"maxTokens":     reasoningOutput,
						"reasoning":     true,
						"input":         []string{"text"},
						"cost": map[string]any{
							"input":      0,
							"output":     0,
							"cacheRead":  0,
							"cacheWrite": 0,
						},
					},
					{
						"id":            codingModel,
						"name":          "MiniMax M2.5",
						"contextWindow": codingContext,
						"maxTokens":     codingOutput,
						"reasoning":     false,
						"input":         []string{"text"},
						"cost": map[string]any{
							"input":      0,
							"output":     0,
							"cacheRead":  0,
							"cacheWrite": 0,
						},
					},
				},
			},
		},
	}

	if err := config.WriteJSON(modelsPath, modelsContent); err != nil {
		return fmt.Errorf("write models.json: %w", err)
	}

	prefsPath := filepath.Join(gsdDir, "preferences.md")
	prefsContent := fmt.Sprintf(`---
version: 1
models:
  research: castai/%s
  planning: castai/%s
  execution: castai/%s
  completion: castai/%s
token_profile: balanced
skill_discovery: suggest
git:
  isolation: worktree
  merge_strategy: squash
---
`, codingModel, reasoningModel, codingModel, codingModel)

	if err := os.WriteFile(prefsPath, []byte(prefsContent), 0644); err != nil {
		return fmt.Errorf("write preferences.md: %w", err)
	}

	return nil
}
