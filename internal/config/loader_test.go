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

	if err := SetTelemetryEnabled(false); err != nil {
		t.Fatalf("disable: %v", err)
	}

	cfg, err = Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.DeviceID != "" {
		t.Errorf("expected empty device ID after disable, got %q", cfg.DeviceID)
	}
}

func TestSetTelemetryEnabled_PreservesDeviceIDOnEnable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Set up a device ID directly via config
	cfg, err := Load()
	require.NoError(t, err)
	cfg.DeviceID = "test-device-id"
	require.NoError(t, Save(cfg))

	if err := SetTelemetryEnabled(true); err != nil {
		t.Fatalf("enable: %v", err)
	}

	cfg, err = Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.DeviceID != "test-device-id" {
		t.Errorf("expected device ID %q preserved, got %q", "test-device-id", cfg.DeviceID)
	}
}
