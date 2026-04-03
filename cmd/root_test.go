package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/castai/kimchi/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecute_DoesNotOverwriteCorruptedConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configDir := filepath.Join(home, ".config", "kimchi")
	require.NoError(t, os.MkdirAll(configDir, 0755))
	corruptedContent := []byte(`{invalid json`)
	configPath := filepath.Join(configDir, "config.json")
	require.NoError(t, os.WriteFile(configPath, corruptedContent, 0600))

	_ = Execute("version")

	got, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, corruptedContent, got, "corrupted config should not be overwritten")
}

func TestExecute_TelemetryEnabledByDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	err := Execute("version")
	require.NoError(t, err)

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.True(t, cfg.TelemetryNoticeShown, "telemetry notice should have been shown")
	assert.NotEmpty(t, cfg.DeviceID, "device ID should have been generated")
}
