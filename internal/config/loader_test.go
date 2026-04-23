package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsTelemetryEnabled_InvalidEnvVar_ReturnsError(t *testing.T) {
	t.Setenv(EnvTelemetry, "banana")

	enabled, err := IsTelemetryEnabled()
	require.Error(t, err, "expected error for invalid KIMCHI_TELEMETRY value")
	assert.False(t, enabled, "expected fail-closed (false) on invalid value")
}

func TestSetTelemetryEnabled_ClearsDeviceIDOnDisable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Set up a device ID directly via config
	cfg, err := Load()
	require.NoError(t, err)
	cfg.DeviceID = "test-device-id"
	require.NoError(t, Save(cfg))

	require.NoError(t, SetTelemetryEnabled(false))

	cfg, err = Load()
	require.NoError(t, err)
	assert.Empty(t, cfg.DeviceID, "expected empty device ID after disable")
}

func TestSetTelemetryEnabled_PreservesDeviceIDOnEnable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Set up a device ID directly via config
	cfg, err := Load()
	require.NoError(t, err)
	cfg.DeviceID = "test-device-id"
	require.NoError(t, Save(cfg))

	require.NoError(t, SetTelemetryEnabled(true))

	cfg, err = Load()
	require.NoError(t, err)
	assert.Equal(t, "test-device-id", cfg.DeviceID, "expected device ID preserved after enable")
}

// harnessTelemetry and harnessConfig mirror the shape the TS harness writes
// into the shared config.json. Fields mix kimchi-owned keys (snake_case) with
// harness-owned keys (camelCase) so tests can seed a realistic on-disk state.
type harnessTelemetry struct {
	Enabled  bool              `json:"enabled"`
	Endpoint string            `json:"endpoint"`
	Headers  map[string]string `json:"headers"`
}

type harnessConfig struct {
	APIKey             string           `json:"api_key"`
	Mode               string           `json:"mode,omitempty"`
	GSDInstalledFor    []string         `json:"gsd_installed_for,omitempty"`
	Telemetry          harnessTelemetry `json:"telemetry"`
	SkillPaths         []string         `json:"skillPaths"`
	MigrationState     string           `json:"migrationState"`
	MaxToolResultChars int              `json:"maxToolResultChars"`
}

func seedHarnessConfig(t *testing.T, home string, cfg harnessConfig) string {
	t.Helper()
	path := filepath.Join(home, ".config", "kimchi", "config.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0600))
	return path
}

// Save must not clobber keys the TS harness writes to the shared config
// (telemetry, skillPaths, migrationState). Losing migrationState made the
// harness re-run its first-run migration on every kimchi write.
func TestSave_PreservesForeignKeys(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := harnessConfig{
		APIKey: "original",
		Telemetry: harnessTelemetry{
			Enabled:  true,
			Endpoint: "https://api.cast.ai/ai-optimizer/v1beta/logs:ingest",
			Headers:  map[string]string{"Authorization": "Bearer foreign"},
		},
		SkillPaths:         []string{".config/kimchi/harness/skills", ".claude/skills"},
		MigrationState:     "done",
		MaxToolResultChars: 20000,
	}
	path := seedHarnessConfig(t, home, cfg)

	require.NoError(t, SetAPIKey("updated"))

	// Same harness state, but with the kimchi-owned fields updated: api_key
	// rotated, and mode back-filled to "override" by Load() at loader.go:47.
	cfg.APIKey = "updated"
	cfg.Mode = string(ModeOverride)
	want, err := json.Marshal(cfg)
	require.NoError(t, err)

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.JSONEq(t, string(want), string(got))
}

// Save must work on a fresh install where the config file does not exist yet:
// ReadJSON returns an empty map, and the merge degenerates to "just write cfg".
func TestSave_FirstSaveWithoutExistingFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	require.NoError(t, SetAPIKey("fresh"))

	path := filepath.Join(home, ".config", "kimchi", "config.json")
	got, err := ReadJSON(path)
	require.NoError(t, err)
	assert.Equal(t, "fresh", got["api_key"])
}

// Clearing a kimchi-owned field must remove its key from disk (delete before
// merge + omitempty on the marshaled cfg) while leaving foreign keys intact.
func TestSave_ClearingKimchiFieldPreservesForeignKeys(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := harnessConfig{
		APIKey:          "original",
		GSDInstalledFor: []string{"opencode"},
		SkillPaths:      []string{".claude/skills"},
		MigrationState:  "done",
	}
	path := seedHarnessConfig(t, home, cfg)

	require.NoError(t, SaveGSDInstalled(nil))

	got, err := ReadJSON(path)
	require.NoError(t, err)
	assert.NotContains(t, got, "gsd_installed_for", "cleared kimchi key should be removed from disk")
	assert.Equal(t, "original", got["api_key"], "untouched kimchi key should survive")
	assert.Equal(t, "done", got["migrationState"], "foreign key should survive")
	assert.Equal(t, []any{".claude/skills"}, got["skillPaths"], "foreign key should survive")
}
