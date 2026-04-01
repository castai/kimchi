package config

import (
	"testing"

	"github.com/google/uuid"
)

func TestGetOrCreateDeviceID_GeneratesAndPersists(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	id1, err := GetOrCreateDeviceID()
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if _, err := uuid.Parse(id1); err != nil {
		t.Fatalf("not a valid UUID: %q", id1)
	}

	id2, err := GetOrCreateDeviceID()
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if id1 != id2 {
		t.Errorf("expected same ID on second call, got %q and %q", id1, id2)
	}
}

func TestGetOrCreateDeviceID_RegeneratesAfterClear(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	id1, err := GetOrCreateDeviceID()
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	cfg.DeviceID = ""
	if err := Save(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	id2, err := GetOrCreateDeviceID()
	if err != nil {
		t.Fatalf("after clear: %v", err)
	}
	if id1 == id2 {
		t.Error("expected new ID after clear, got same")
	}
}

func TestSetTelemetryEnabled_ClearsDeviceIDOnDisable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Generate a device ID first
	if _, err := GetOrCreateDeviceID(); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := SetTelemetryEnabled(false); err != nil {
		t.Fatalf("disable: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.DeviceID != "" {
		t.Errorf("expected empty device ID after disable, got %q", cfg.DeviceID)
	}
}

func TestSetTelemetryEnabled_PreservesDeviceIDOnEnable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	id, err := GetOrCreateDeviceID()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := SetTelemetryEnabled(true); err != nil {
		t.Fatalf("enable: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.DeviceID != id {
		t.Errorf("expected device ID %q preserved, got %q", id, cfg.DeviceID)
	}
}
