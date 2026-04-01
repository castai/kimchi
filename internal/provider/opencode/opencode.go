package opencode

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/castai/kimchi/internal/config"
	"github.com/castai/kimchi/internal/gsd"
	"github.com/castai/kimchi/internal/tools"
)

// Env prepares the environment for launching OpenCode with Cast AI
// configuration. It writes a managed config file to ~/.config/kimchi/opencode/opencode.json
// and returns environment variables that redirect OpenCode to use it.
func Env(apiKey string) (map[string]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	userOCDir := filepath.Join(homeDir, ".config", "opencode")
	managedOCDir := filepath.Join(homeDir, ".config", "kimchi", "opencode")

	// Copy the user's entire opencode directory first (agents, commands,
	// skills, rules, etc.) so custom user content is carried over.
	// Then GSD files already in managedOCDir (installed directly there)
	// are preserved since copyDir does not delete existing files.
	if info, err := os.Stat(userOCDir); err == nil && info.IsDir() {
		if err := gsd.CopyInstallation(userOCDir, managedOCDir); err != nil {
			return nil, fmt.Errorf("copy user opencode config: %w", err)
		}
	}

	// Merge kimchi provider into the config JSON.
	userConfigPath := filepath.Join(userOCDir, "opencode.json")
	existing, err := config.ReadJSON(userConfigPath)
	if err != nil {
		return nil, fmt.Errorf("read existing config: %w", err)
	}

	existing["$schema"] = "https://opencode.ai/config.json"

	providers, _ := existing["provider"].(map[string]any)
	if providers == nil {
		providers = make(map[string]any)
	}
	providers[tools.ProviderName] = tools.OpenCodeProviderConfig(apiKey)
	existing["provider"] = providers

	if _, ok := existing["compaction"]; !ok {
		existing["compaction"] = map[string]any{
			"auto": true,
		}
	}

	managedConfigPath := filepath.Join(managedOCDir, "opencode.json")
	if err := config.WriteJSON(managedConfigPath, existing); err != nil {
		return nil, fmt.Errorf("write managed config: %w", err)
	}

	return map[string]string{
		"XDG_CONFIG_HOME": filepath.Join(homeDir, ".config", "kimchi"),
	}, nil
}
