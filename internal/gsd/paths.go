package gsd

import (
	"os"
	"path/filepath"
	"strings"
)

// resolveOpenCodeGlobalRoot returns the global OpenCode configuration root
// using the same priority chain as gsd-opencode and get-shit-done-cc:
//
//  1. OPENCODE_CONFIG_DIR – explicit override
//  2. dirname(OPENCODE_CONFIG) – when a specific config file is pointed to
//  3. XDG_CONFIG_HOME/opencode – XDG-compliant systems
//  4. ~/.config/opencode – default
func resolveOpenCodeGlobalRoot() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if v := os.Getenv("OPENCODE_CONFIG_DIR"); v != "" {
		return expandTilde(v, homeDir), nil
	}
	if v := os.Getenv("OPENCODE_CONFIG"); v != "" {
		return filepath.Dir(expandTilde(v, homeDir)), nil
	}
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return filepath.Join(expandTilde(v, homeDir), "opencode"), nil
	}
	return filepath.Join(homeDir, ".config", "opencode"), nil
}

// configRoot returns the tool's configuration root for the given scope.
// For project scope this is always a dotdir under cwd; for global scope
// it uses the standard tool location (OpenCode respects env vars).
func configRoot(installType InstallationType, scope string) (string, error) {
	if scope == "project" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		switch installType {
		case InstallationOpenCode:
			return filepath.Join(cwd, ".opencode"), nil
		case InstallationCodex:
			return filepath.Join(cwd, ".codex"), nil
		}
	}

	switch installType {
	case InstallationOpenCode:
		return resolveOpenCodeGlobalRoot()
	case InstallationCodex:
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(homeDir, ".codex"), nil
	}
	return "", nil
}

// gsdCommandDirs returns the candidate GSD command directories under a config
// root in preference order: current structure first, then legacy (no trailing s).
func gsdCommandDirs(root string) []string {
	return []string{
		filepath.Join(root, "commands", "gsd"),
		filepath.Join(root, "command", "gsd"),
	}
}

// expandTilde replaces a leading ~ with homeDir.
func expandTilde(path, homeDir string) string {
	if path == "~" {
		return homeDir
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, path[2:])
	}
	return path
}
