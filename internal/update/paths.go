package update

import (
	"fmt"
	"os"
	"path/filepath"
)

// cacheDir returns the base cache directory, respecting XDG_CACHE_HOME.
func cacheDir() (string, error) {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, ".cache"), nil
}

// dataDir returns the base data directory, respecting XDG_DATA_HOME.
func dataDir() (string, error) {
	dir := os.Getenv("XDG_DATA_HOME")
	if dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share"), nil
}

// backupDir returns the directory used to store pre-update binary backups.
func backupDir() (string, error) {
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appDir, "backups"), nil
}

// ResolveExecutablePath returns the real path of the current executable, resolving symlinks.
func ResolveExecutablePath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", fmt.Errorf("resolve symlinks for %s: %w", execPath, err)
	}
	return resolved, nil
}
