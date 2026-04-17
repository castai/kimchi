package update

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/klog/v2"
)

// homeDir returns the user's home directory. It prefers os.UserHomeDir and
// falls back to the HOME environment variable. On any Unix system where a
// user is running a shell, one of these will succeed.
func homeDir() string {
	if d, err := os.UserHomeDir(); err == nil {
		return d
	}
	home := os.Getenv("HOME")
	klog.V(1).InfoS("os.UserHomeDir failed, falling back to HOME env var", "HOME", home)
	return home
}

// cacheDir returns the base cache directory, respecting XDG_CACHE_HOME.
func cacheDir() string {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return dir
	}
	return filepath.Join(homeDir(), ".cache")
}

// dataDir returns the base data directory, respecting XDG_DATA_HOME.
func dataDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir
	}
	return filepath.Join(homeDir(), ".local", "share")
}

// harnessDataDir returns the directory for harness supporting files
// (package.json, theme/), following the XDG Base Directory specification.
func harnessDataDir() string {
	return filepath.Join(dataDir(), "kimchi")
}

// backupDir returns the directory used to store pre-update binary backups.
func backupDir() string {
	return filepath.Join(cacheDir(), appDir, "backups")
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
