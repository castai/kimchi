package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	// Default to override mode for configs created before inject mode existed.
	if cfg.Mode == "" {
		cfg.Mode = ModeOverride
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

// ResolveAPIKey returns the API key from the config, overridden by
// KIMCHI_API_KEY env var. Returns an error if no key is available.
func ResolveAPIKey(cfg *Config) (string, error) {
	apiKey := cfg.APIKey
	if envKey := os.Getenv(envAPIKey); envKey != "" {
		apiKey = envKey
	}
	if apiKey == "" {
		return "", fmt.Errorf("no API key configured — run 'kimchi' to set up, or set KIMCHI_API_KEY")
	}
	return apiKey, nil
}

func SetAPIKey(key string) error {
	cfg, err := Load()
	if err != nil {
		return fmt.Errorf("load existing config: %w", err)
	}

	cfg.APIKey = key

	return Save(cfg)
}

func SaveModelConfig(mainSlug, codingSlug, subSlug string) error {
	cfg, err := Load()
	if err != nil {
		return fmt.Errorf("load existing config: %w", err)
	}

	cfg.ModelMain = mainSlug
	cfg.ModelCoding = codingSlug
	cfg.ModelSub = subSlug

	return Save(cfg)
}

func SaveGSDInstalled(tools []string) error {
	cfg, err := Load()
	if err != nil {
		return fmt.Errorf("load existing config: %w", err)
	}

	cfg.GSDInstalledFor = tools

	return Save(cfg)
}

const EnvTelemetry = "KIMCHI_TELEMETRY"

// IsTelemetryEnabledFromConfig returns whether telemetry is enabled using a pre-loaded config.
// Checks environment variable first, then the provided config. No I/O.
func IsTelemetryEnabledFromConfig(cfg *Config) (bool, error) {
	if envVal := os.Getenv(EnvTelemetry); envVal != "" {
		enabled, err := ParseSwitch(envVal)
		if err != nil {
			return false, fmt.Errorf("invalid %s value: %w", EnvTelemetry, err)
		}
		return enabled, nil
	}
	if cfg.TelemetryEnabled == nil {
		return true, nil
	}
	return *cfg.TelemetryEnabled, nil
}

// IsTelemetryEnabled returns whether telemetry is enabled.
// Checks environment variable first, then config file.
// Returns (enabled, error). If env var is set but invalid, returns error.
func IsTelemetryEnabled() (bool, error) {
	cfg, err := Load()
	if err != nil {
		return false, err
	}
	return IsTelemetryEnabledFromConfig(cfg)
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
		return false, fmt.Errorf("invalid switch value: %q (expected on/off, true/false, 1/0, yes/no)", s)
	}
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
