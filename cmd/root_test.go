package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/castai/kimchi/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfigFile(t *testing.T, homeDir string, content []byte) string {
	t.Helper()
	configDir := filepath.Join(homeDir, ".config", "kimchi")
	require.NoError(t, os.MkdirAll(configDir, 0755))
	configPath := filepath.Join(configDir, "config.json")
	require.NoError(t, os.WriteFile(configPath, content, 0600))
	return configPath
}

func TestExecute_DoesNotOverwriteCorruptedConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	corruptedContent := []byte(`{invalid json`)
	configPath := writeConfigFile(t, home, corruptedContent)

	err := Execute("version")

	require.NoError(t, err)
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

func TestExecute_TelemetryDisabledViaEnvVar_DoesNotGenerateDeviceID(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(config.EnvTelemetry, "false")

	err := Execute("config", "telemetry", "off")
	require.NoError(t, err)

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Empty(t, cfg.DeviceID, "device ID should not be persisted when telemetry is disabled via env var")
	assert.False(t, cfg.TelemetryNoticeShown, "telemetry notice should not be shown when disabled via env var")
}

func TestExecute_LegacyConfigWithoutTelemetryFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	writeConfigFile(t, home, []byte(`{"api_key": "test-key"}`))

	err := Execute("version")
	require.NoError(t, err)

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "test-key", cfg.APIKey, "existing API key should be preserved")
	assert.NotEmpty(t, cfg.DeviceID, "device ID should have been generated")
	assert.True(t, cfg.TelemetryNoticeShown, "telemetry notice should have been shown")
}
