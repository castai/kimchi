package codex

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/castai/kimchi/internal/config"
	"github.com/castai/kimchi/internal/gsd"
	"github.com/castai/kimchi/internal/tools"
)

// Env prepares the environment for launching Codex with Cast AI configuration.
// It copies the user's ~/.codex/ to a kimchi-managed directory, merges the
// kimchi provider config, and returns CODEX_HOME pointing to the managed copy
// so the original user directory is never modified.
func Env(apiKey string) (map[string]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	userCodexDir := filepath.Join(homeDir, ".codex")
	managedCodexDir := filepath.Join(homeDir, ".config", "kimchi", "codex")

	// Copy only config-relevant content from the user's codex directory.
	// We skip runtime artifacts (sqlite DBs, logs, sessions, .tmp, etc.)
	// that can contain read-only files and aren't needed for config.
	codexConfigDirs := []string{"agents", "skills", "get-shit-done", "hooks"}
	codexConfigFiles := []string{"AGENTS.md", "AGENTS.override.md"}
	for _, dir := range codexConfigDirs {
		src := filepath.Join(userCodexDir, dir)
		if info, err := os.Stat(src); err == nil && info.IsDir() {
			if err := gsd.CopyInstallation(src, filepath.Join(managedCodexDir, dir)); err != nil {
				return nil, fmt.Errorf("copy user codex %s: %w", dir, err)
			}
		}
	}
	for _, file := range codexConfigFiles {
		src := filepath.Join(userCodexDir, file)
		dst := filepath.Join(managedCodexDir, file)
		if data, err := os.ReadFile(src); err == nil {
			if err := config.WriteFile(dst, data); err != nil {
				return nil, fmt.Errorf("copy user codex %s: %w", file, err)
			}
		}
	}

	// Ensure the managed directory exists even if the user has no ~/.codex/.
	if err := os.MkdirAll(managedCodexDir, 0755); err != nil {
		return nil, fmt.Errorf("create managed codex dir: %w", err)
	}

	// Copy the user's config.toml as a base so custom settings (sandbox policies,
	// approval settings, other providers) are preserved during the wrapped run.
	managedConfigPath := filepath.Join(managedCodexDir, "config.toml")
	userConfigPath := filepath.Join(userCodexDir, "config.toml")
	if data, err := os.ReadFile(userConfigPath); err == nil {
		if err := config.WriteFile(managedConfigPath, data); err != nil {
			return nil, fmt.Errorf("copy user codex config.toml: %w", err)
		}
	}

	// Merge kimchi provider into the managed config (on top of user's settings).
	if err := writeKimchiProvider(managedConfigPath); err != nil {
		return nil, fmt.Errorf("write managed codex config: %w", err)
	}

	// Write model catalog into the managed directory.
	managedCatalogPath := filepath.Join(managedCodexDir, "kimchi-models.json")
	if err := tools.WriteCodexModelCatalog(managedCatalogPath); err != nil {
		return nil, fmt.Errorf("write model catalog: %w", err)
	}

	// Write AGENTS.md only if it doesn't exist in the managed dir.
	agentsPath := filepath.Join(managedCodexDir, "AGENTS.md")
	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		if err := config.WriteFile(agentsPath, []byte(tools.CodexAgentMD())); err != nil {
			return nil, fmt.Errorf("write AGENTS.md: %w", err)
		}
	}

	return map[string]string{
		tools.APIKeyEnv: apiKey,
		"CODEX_HOME":    managedCodexDir,
	}, nil
}

// writeKimchiProvider merges the kimchi provider into the managed config.toml.
func writeKimchiProvider(configPath string) error {
	cfg, err := config.ReadTOML(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	cfg["model"] = tools.CodingModel.Slug
	cfg["model_provider"] = tools.ProviderName()
	cfg["suppress_unstable_features_warning"] = true

	providers, ok := cfg["model_providers"].(map[string]any)
	if !ok {
		providers = make(map[string]any)
		cfg["model_providers"] = providers
	}

	providers["kimchi"] = map[string]any{
		"name":                 "Kimchi by Cast AI",
		"base_url":             tools.BaseURL(),
		"env_key":              tools.APIKeyEnv,
		"env_key_instructions": tools.EnvKeyInstructions(),
		"wire_api":             "responses",
	}

	// Reference the catalog from the same managed directory.
	cfg["model_catalog_json"] = filepath.Join(filepath.Dir(configPath), "kimchi-models.json")

	return config.WriteTOML(configPath, cfg)
}
