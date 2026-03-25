package update

import (
	"fmt"
	"os"
	"path/filepath"
)

// configDir returns the base configuration directory, respecting XDG_CONFIG_HOME.
func configDir() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, ".config"), nil
}

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
