package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/castai/kimchi/internal/config"
)

func TestWriteCodex(t *testing.T) {
	t.Run("without make default skips top-level keys", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)

		err := WriteCodex(config.ScopeGlobal, false)
		require.NoError(t, err)

		cfg, err := config.ReadTOML(filepath.Join(tmpDir, ".codex", "config.toml"))
		require.NoError(t, err)

		assert.Nil(t, cfg["model"], "model should not be set when makeDefault is false")
		assert.Nil(t, cfg["model_provider"], "model_provider should not be set when makeDefault is false")

		providers := cfg["model_providers"].(map[string]any)
		_, ok := providers["kimchi"]
		assert.True(t, ok, "kimchi provider should still be added")
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

		err := WriteCodex(config.ScopeGlobal, true)
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

		err := WriteCodex(config.ScopeGlobal, true)
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

	t.Run("does not overwrite existing AGENTS.md", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)

		agentsDir := filepath.Join(tmpDir, ".codex")
		require.NoError(t, os.MkdirAll(agentsDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "AGENTS.md"), []byte("custom instructions"), 0644))

		err := WriteCodex(config.ScopeGlobal, true)
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(agentsDir, "AGENTS.md"))
		require.NoError(t, err)
		assert.Equal(t, "custom instructions", string(content))
	})
}
