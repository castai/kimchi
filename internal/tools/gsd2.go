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

func writeGSD2(scope config.ConfigScope, apiKey string, models ModelConfig) error {
	if apiKey == "" {
		return fmt.Errorf("API key not configured")
	}

	modelsPath, err := config.ScopePaths(scope, "~/.gsd/agent/models.json")
	if err != nil {
		return fmt.Errorf("get models config path: %w", err)
	}
	var modelEntries []map[string]any
	for _, m := range models.All {
		entry := map[string]any{
			"id":            m.Slug,
			"name":          m.DisplayName,
			"contextWindow": m.Limits.ContextWindow,
			"maxTokens":     m.Limits.MaxOutputTokens,
			"reasoning":     m.Reasoning,
			"input":         m.InputModalities,
			"cost": map[string]any{
				"input":      0,
				"output":     0,
				"cacheRead":  0,
				"cacheWrite": 0,
			},
		}
		modelEntries = append(modelEntries, entry)
	}

	modelsContent := map[string]any{
		"providers": map[string]any{
			"kimchi": map[string]any{
				"name":         "Kimchi",
				"baseUrl":      baseURL,
				"apiKey":       apiKey,
				"api":          "openai-completions",
				"defaultModel": models.Main.Slug,
				"models":       modelEntries,
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
  research: kimchi/%s
  planning: kimchi/%s
  execution: kimchi/%s
  completion: kimchi/%s
token_profile: balanced
skill_discovery: suggest
git:
  isolation: worktree
  merge_strategy: squash
---
`, models.Main.Slug, models.Main.Slug, models.Coding.Slug, models.Coding.Slug)

	if err := config.WriteFile(prefsPath, []byte(prefsContent)); err != nil {
		return fmt.Errorf("write preferences.md: %w", err)
	}

	return nil
}
