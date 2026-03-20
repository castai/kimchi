package tools

import (
	"fmt"
	"os/exec"

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

	modelsPath, err := config.ScopePaths(scope, "~/.gsd/agent/models.json")
	if err != nil {
		return fmt.Errorf("get models config path: %w", err)
	}
	modelsContent := map[string]any{
		"providers": map[string]any{
			"castai": map[string]any{
				"name":         "Cast AI",
				"baseUrl":      baseURL,
				"apiKey":       apiKey,
				"api":          "openai-completions",
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
					{
						"id":            imageModel,
						"name":          "Kimi K2.5",
						"contextWindow": imageContext,
						"maxTokens":     imageOutput,
						"reasoning":     false,
						"input":         []string{"text", "image"},
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

	prefsPath, err := config.ScopePaths(scope, "~/.gsd/preferences.md")
	if err != nil {
		return fmt.Errorf("get preferences config path: %w", err)
	}
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

	if err := config.WriteFile(prefsPath, []byte(prefsContent)); err != nil {
		return fmt.Errorf("write preferences.md: %w", err)
	}

	return nil
}
