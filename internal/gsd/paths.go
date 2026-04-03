package gsd

import (
	"errors"
	"fmt"
	"io/fs"
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

// KimchiManagedPath returns the kimchi-managed GSD directory for the given
// installation type (global scope only).
func KimchiManagedPath(installType InstallationType) (string, error) {
	var toolName string
	switch installType {
	case InstallationClaudeCode:
		toolName = "claude-code"
	case InstallationOpenCode:
		toolName = "opencode"
	case InstallationCodex:
		toolName = "codex"
	default:
		return "", fmt.Errorf("unsupported installation type: %s", installType)
	}
	return getGSDPath(toolName, "global")
}

// EnsureSymlink creates a symlink at target pointing to src. If target already
// exists as a real file or directory, it is left untouched.
// On Windows, if symlink creation fails (e.g. no Developer Mode), it falls back
// to copying the directory contents.
func EnsureSymlink(src, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	err := os.Symlink(src, target)
	if err == nil {
		return nil
	}
	if !errors.Is(err, fs.ErrExist) {
		if copyErr := CopyInstallation(src, target); copyErr != nil {
			return fmt.Errorf("create GSD symlink: %w (copy fallback also failed: %v)", err, copyErr)
		}
		return nil
	}

	existing, readErr := os.Readlink(target)
	if readErr != nil {
		// Not a symlink (real file/dir) — leave untouched.
		return nil
	}
	if existing == src {
		return nil
	}
	// Stale symlink — replace it.
	if err := os.Remove(target); err != nil {
		return fmt.Errorf("remove stale symlink: %w", err)
	}
	if err := os.Symlink(src, target); err != nil {
		if copyErr := CopyInstallation(src, target); copyErr != nil {
			return fmt.Errorf("create GSD symlink: %w (copy fallback also failed: %v)", err, copyErr)
		}
	}
	return nil
}

// CopyInstallation copies GSD files from src to dst.
func CopyInstallation(src, dst string) error {
	return copyDir(src, dst)
}

// ReadAgentFiles reads all agent files from dir and returns them.
func ReadAgentFiles(dir string) ([]AgentFile, error) {
	d := &Detector{}
	return d.agentFiles(dir)
}
