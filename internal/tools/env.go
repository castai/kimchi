package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// MergeEnv builds an environment slice from the current process environment
// with the provided overrides applied.
func MergeEnv(env map[string]string) []string {
	existing := os.Environ()
	merged := make([]string, 0, len(existing)+len(env))
	for _, e := range existing {
		key, _, _ := strings.Cut(e, "=")
		if _, override := env[key]; !override {
			merged = append(merged, e)
		}
	}
	for k, v := range env {
		merged = append(merged, k+"="+v)
	}
	return merged
}

// FindBinary locates a binary by name. It checks PATH first via LookPath,
// then falls back to common installation directories (npm global, nvm, ~/.local/bin,
// ~/.<tool>/bin) so tools installed outside the current PATH are still found.
func FindBinary(name string) (string, error) {
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("%s is not installed or not in PATH", name)
	}

	// Tool-specific install directories (e.g. ~/.opencode/bin/opencode).
	candidates := []string{
		filepath.Join(homeDir, "."+name, "bin", name),
		filepath.Join(homeDir, ".local", "bin", name),
	}

	// nvm-managed node global bins.
	nvmDir := filepath.Join(homeDir, ".nvm", "versions", "node")
	if entries, err := os.ReadDir(nvmDir); err == nil {
		for i := len(entries) - 1; i >= 0; i-- {
			candidates = append(candidates, filepath.Join(nvmDir, entries[i].Name(), "bin", name))
		}
	}

	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, nil
		}
	}

	return "", fmt.Errorf("%s is not installed or not in PATH", name)
}

// ExitError is returned by RunTool to propagate the child process exit code
// without calling os.Exit directly, allowing deferred cleanup to run.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit status %d", e.Code)
}
