package config

import (
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
