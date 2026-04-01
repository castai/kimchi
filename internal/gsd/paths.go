package gsd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// KimchiManagedPath returns the kimchi-managed GSD directory for the given
// installation type (global scope only).
func KimchiManagedPath(installType InstallationType) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	switch installType {
	case InstallationClaudeCode:
		return filepath.Join(homeDir, ".config", "kimchi", "claude-code"), nil
	case InstallationOpenCode:
		return filepath.Join(homeDir, ".config", "kimchi", "opencode"), nil
	case InstallationCodex:
		return filepath.Join(homeDir, ".config", "kimchi", "codex"), nil
	default:
		return "", fmt.Errorf("unsupported installation type: %s", installType)
	}
}

// EnsureSymlink creates a symlink at target pointing to src. If target already
// exists as a real file or directory, it is left untouched.
func EnsureSymlink(src, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	err := os.Symlink(src, target)
	if err == nil {
		return nil
	}
	if !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("create GSD symlink: %w", err)
	}

	// Target exists. If it's a symlink pointing to the wrong place, update it.
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
		return fmt.Errorf("create GSD symlink: %w", err)
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
	return d.getAgentFiles(dir)
}
