package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

func SavePreferences(apiKey string, mode ConfigMode, selectedTools []string, scope string, telemetryOptIn bool) error {
	cfg, err := Load()
	if err != nil {
		return fmt.Errorf("load existing config: %w", err)
	}

	cfg.APIKey = apiKey
	cfg.Mode = mode
	cfg.SelectedTools = selectedTools
	cfg.Scope = scope
	cfg.TelemetryOptIn = telemetryOptIn

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
