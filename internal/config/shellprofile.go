package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"
)

// ExportEnvToShellProfile writes an export line for the given environment variable key=value to the
// user's shell profile. It detects the shell from $SHELL and falls back to checking which profile
// files exist. Returns the path written to, or ("", nil) if no profile was detected. An error is
// returned only for real I/O failures (permissions, symlinks, write errors), not for missing profiles.
func ExportEnvToShellProfile(key, value string) (string, error) {
	profilePath, shell := detectShellProfile()
	if profilePath == "" {
		return "", nil
	}

	// Resolve symlinks so we read/write the real file and don't replace
	// a symlink with a regular file via tmp+rename.
	resolved, err := filepath.EvalSymlinks(profilePath)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("resolve symlink %s: %w", profilePath, err)
	}
	if err == nil {
		profilePath = resolved
	}

	var exportLine string
	var matchPrefix string
	if shell == "fish" {
		exportLine = fmt.Sprintf("set -gx %s %s", key, value)
		matchPrefix = fmt.Sprintf("set -gx %s ", key)
	} else {
		exportLine = fmt.Sprintf("export %s=%s", key, value)
		matchPrefix = fmt.Sprintf("export %s=", key)
	}

	content, err := os.ReadFile(profilePath)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("read %s: %w", profilePath, err)
	}

	if len(content) > 0 && !utf8.Valid(content) {
		return "", fmt.Errorf("shell profile %s contains non-UTF-8 content, skipping", profilePath)
	}

	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), matchPrefix) {
			lines[i] = exportLine
			found = true
			break
		}
	}

	if !found {
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines[len(lines)-1] = exportLine
			lines = append(lines, "")
		} else {
			lines = append(lines, exportLine)
		}
	}

	newContent := strings.Join(lines, "\n")
	if err := WriteFile(profilePath, []byte(newContent)); err != nil {
		return "", fmt.Errorf("write %s: %w", profilePath, err)
	}

	return profilePath, nil
}

func detectShellProfile() (string, string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ""
	}

	shell := filepath.Base(os.Getenv("SHELL"))
	switch shell {
	case "zsh":
		return filepath.Join(home, ".zshrc"), "zsh"
	case "bash":
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, ".bash_profile"), "bash"
		}
		return filepath.Join(home, ".bashrc"), "bash"
	case "fish":
		return filepath.Join(home, ".config", "fish", "config.fish"), "fish"
	}

	// Fallback: check which profile files exist
	if runtime.GOOS == "darwin" {
		if _, err := os.Stat(filepath.Join(home, ".zshrc")); err == nil {
			return filepath.Join(home, ".zshrc"), "zsh"
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".bashrc")); err == nil {
		return filepath.Join(home, ".bashrc"), "bash"
	}
	if _, err := os.Stat(filepath.Join(home, ".bash_profile")); err == nil {
		return filepath.Join(home, ".bash_profile"), "bash"
	}

	return "", ""
}
