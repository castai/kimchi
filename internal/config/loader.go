package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

const (
	envAPIKey    = "KIMCHI_API_KEY"
	configDir    = ".config"
	appConfigDir = "kimchi"
	configFile   = "config.json"
)

func ConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, configDir, appConfigDir, configFile)
}

func Load() (*Config, error) {
	path := ConfigPath()
	if path == "" {
		return &Config{}, fmt.Errorf("get config path: home directory not found")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	return &cfg, nil
}

func GetAPIKey() (string, error) {
	if key := os.Getenv(envAPIKey); key != "" {
		return key, nil
	}

	cfg, err := Load()
	if err != nil {
		return "", err
	}

	return cfg.APIKey, nil
}

func SetAPIKey(key string) error {
	cfg, err := Load()
	if err != nil {
		return fmt.Errorf("load existing config: %w", err)
	}

	cfg.APIKey = key

	return Save(cfg)
}

const envTelemetry = "KIMCHI_TELEMETRY"

// IsTelemetryEnabled returns whether telemetry is enabled.
// Checks environment variable first, then config file.
// Returns (enabled, error). If env var is set but invalid, returns error.
func IsTelemetryEnabled() (enabled bool, err error) {
	// Check environment variable first
	if envVal := os.Getenv(envTelemetry); envVal != "" {
		enabled, err := ParseSwitch(envVal)
		if err != nil {
			return false, fmt.Errorf("invalid %s value: %w", envTelemetry, err)
		}
		return enabled, nil
	}

	// Fall back to config
	cfg, err := Load()
	if err != nil {
		return false, err
	}

	// If not set (nil), default to enabled (opt-out)
	if cfg.TelemetryEnabled == nil {
		return true, nil
	}

	return *cfg.TelemetryEnabled, nil
}

func SetTelemetryEnabled(enabled bool) error {
	cfg, err := Load()
	if err != nil {
		return fmt.Errorf("load existing config: %w", err)
	}

	cfg.TelemetryEnabled = &enabled
	if !enabled {
		cfg.DeviceID = ""
	}

	return Save(cfg)
}

// GetOrCreateDeviceID returns the device ID from config, generating a new UUID if empty.
func GetOrCreateDeviceID() (string, error) {
	cfg, err := Load()
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}

	if cfg.DeviceID != "" {
		return cfg.DeviceID, nil
	}

	cfg.DeviceID = uuid.NewString()
	if err := Save(cfg); err != nil {
		return "", fmt.Errorf("save config: %w", err)
	}

	return cfg.DeviceID, nil
}

// ParseSwitch parses a string value as a boolean switch.
// Accepts: "on"|"off", "true"|"false", "1"|"0", "yes"|"no" (case insensitive)
func ParseSwitch(s string) (bool, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "on", "true", "1", "yes":
		return true, nil
	case "off", "false", "0", "no":
		return false, nil
	default:
		// Try strconv as fallback for other valid bool formats
		if b, err := strconv.ParseBool(s); err == nil {
			return b, nil
		}
		return false, fmt.Errorf("invalid switch value: %q (expected on/off, true/false, 1/0, yes/no)", s)
	}
}

// ShouldShowTelemetryNotice returns true if the one-time telemetry disclosure
// notice has not yet been shown and telemetry is currently enabled.
// Returns false when KIMCHI_TELEMETRY env var is set (CI/automated contexts)
// or when telemetry has been explicitly disabled.
func ShouldShowTelemetryNotice() (bool, error) {
	// If the env var is set, the user is in an automated/CI context — skip notice.
	if os.Getenv(envTelemetry) != "" {
		return false, nil
	}

	cfg, err := Load()
	if err != nil {
		return false, err
	}

	if cfg.TelemetryNoticeShown {
		return false, nil
	}

	// If telemetry has been explicitly disabled, no notice needed.
	if cfg.TelemetryEnabled != nil && !*cfg.TelemetryEnabled {
		return false, nil
	}

	return true, nil
}

// MarkTelemetryNoticeShown persists that the telemetry disclosure notice has been shown.
func MarkTelemetryNoticeShown() error {
	cfg, err := Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	cfg.TelemetryNoticeShown = true
	return Save(cfg)
}

func Save(cfg *Config) error {
	path := ConfigPath()
	if path == "" {
		return fmt.Errorf("get config path: home directory not found")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("atomic rename config: %w", err)
	}

	if err := os.Chmod(path, 0600); err != nil {
		return fmt.Errorf("set config permissions: %w", err)
	}

	return nil
}
