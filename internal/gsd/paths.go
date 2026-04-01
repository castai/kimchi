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
		return filepath.Join(homeDir, ".config", "kimchi", "gsd", "claude-code"), nil
	case InstallationOpenCode:
		return filepath.Join(homeDir, ".config", "kimchi", "opencode"), nil
	case InstallationCodex:
		return filepath.Join(homeDir, ".config", "kimchi", "gsd", "codex"), nil
	default:
		return "", fmt.Errorf("unsupported installation type: %s", installType)
	}
}

// EnsureSymlink creates a symlink from target to src. If target already exists
// (symlink or real directory), it is left untouched.
func EnsureSymlink(src, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	err := os.Symlink(src, target)
	if err == nil || errors.Is(err, fs.ErrExist) {
		return nil
	}
	return fmt.Errorf("create GSD symlink: %w", err)
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
