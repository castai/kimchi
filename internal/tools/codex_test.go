package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/castai/kimchi/internal/config"
)

func TestWriteCodex(t *testing.T) {
	t.Run("sets model and provider at top level", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)

		err := writeCodex(config.ScopeGlobal)
		require.NoError(t, err)

		cfg, err := config.ReadTOML(filepath.Join(tmpDir, ".codex", "config.toml"))
		require.NoError(t, err)

		assert.Equal(t, codingModel.slug, cfg["model"])
		assert.Equal(t, providerName, cfg["model_provider"])

		providers := cfg["model_providers"].(map[string]any)
		_, ok := providers["kimchi"]
		assert.True(t, ok, "kimchi provider should be added")
	})

	t.Run("preserves existing config sections", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)

		configDir := filepath.Join(tmpDir, ".codex")
		require.NoError(t, os.MkdirAll(configDir, 0755))

		existing := `[history]
persistence = true
max_entries = 100

[scheduler]
check_interval = 30
`
		require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(existing), 0644))

		err := writeCodex(config.ScopeGlobal)
		require.NoError(t, err)

		cfg, err := config.ReadTOML(filepath.Join(configDir, "config.toml"))
		require.NoError(t, err)

		history := cfg["history"].(map[string]any)
		assert.Equal(t, int64(100), history["max_entries"])

		scheduler := cfg["scheduler"].(map[string]any)
		assert.Equal(t, int64(30), scheduler["check_interval"])
	})

	t.Run("preserves existing providers", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)

		configDir := filepath.Join(tmpDir, ".codex")
		require.NoError(t, os.MkdirAll(configDir, 0755))

		existing := `[model_providers.other]
name = "Other Provider"
base_url = "https://other.example.com"
env_key = "OTHER_API_KEY"
wire_api = "responses"
`
		require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(existing), 0644))

		err := writeCodex(config.ScopeGlobal)
		require.NoError(t, err)

		cfg, err := config.ReadTOML(filepath.Join(configDir, "config.toml"))
		require.NoError(t, err)

		providers := cfg["model_providers"].(map[string]any)
		other := providers["other"].(map[string]any)
		assert.Equal(t, "Other Provider", other["name"])
		assert.Equal(t, "https://other.example.com", other["base_url"])

		_, ok := providers["kimchi"]
		assert.True(t, ok, "kimchi provider should be added alongside existing providers")
	})

	t.Run("writes model catalog with correct metadata", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)

		err := writeCodex(config.ScopeGlobal)
		require.NoError(t, err)

		catalogPath := filepath.Join(tmpDir, ".codex", "kimchi-models.json")
		data, err := os.ReadFile(catalogPath)
		require.NoError(t, err)

		var catalog codexCatalog
		require.NoError(t, json.Unmarshal(data, &catalog))
		require.Len(t, catalog.Models, len(allModels))

		bySlug := make(map[string]codexModelEntry)
		for _, m := range catalog.Models {
			bySlug[m.Slug] = m
		}
		assert.Equal(t, codingModel.limits.contextWindow, bySlug[codingModel.slug].ContextWindow)
		assert.Equal(t, reasoningModel.limits.contextWindow, bySlug[reasoningModel.slug].ContextWindow)
		assert.Equal(t, imageModel.limits.contextWindow, bySlug[imageModel.slug].ContextWindow)

		// Reasoning model should have low/medium/high levels
		assert.Equal(t, "medium", bySlug[reasoningModel.slug].DefaultReasoningLevel)
		assert.Equal(t, 3, len(bySlug[reasoningModel.slug].SupportedReasoningLevels))

		// Non-reasoning models should have "none"
		assert.Equal(t, "none", bySlug[codingModel.slug].DefaultReasoningLevel)
		assert.Equal(t, 1, len(bySlug[codingModel.slug].SupportedReasoningLevels))

		// Verify config.toml references the catalog
		cfg, err := config.ReadTOML(filepath.Join(tmpDir, ".codex", "config.toml"))
		require.NoError(t, err)
		assert.Equal(t, catalogPath, cfg["model_catalog_json"])
	})

	t.Run("does not overwrite existing AGENTS.md", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)

		agentsDir := filepath.Join(tmpDir, ".codex")
		require.NoError(t, os.MkdirAll(agentsDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "AGENTS.md"), []byte("custom instructions"), 0644))

		err := writeCodex(config.ScopeGlobal)
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(agentsDir, "AGENTS.md"))
		require.NoError(t, err)
		assert.Equal(t, "custom instructions", string(content))
	})
}
