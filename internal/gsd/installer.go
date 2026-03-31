package gsd

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Installer struct{}

func NewInstaller() *Installer {
	return &Installer{}
}

type InstallResult struct {
	Type      InstallationType
	Path      string
	Installed []string
}

func (i *Installer) Install(installType InstallationType, scope string) (*InstallResult, error) {
	switch installType {
	case InstallationOpenCode:
		return i.installOpenCode(scope)
	case InstallationClaudeCode:
		return i.installClaudeCode(scope)
	case InstallationCodex:
		return i.installCodex(scope)
	default:
		return nil, fmt.Errorf("unsupported tool for GSD: %s", installType)
	}
}

func (i *Installer) installOpenCode(scope string) (*InstallResult, error) {
	args := []string{"--yes", "gsd-opencode@latest"}
	if scope == "project" {
		args = append(args, "--local")
	} else {
		args = append(args, "--global")
	}

	destPath, err := getOpenCodeGSDPath(scope)
	if err != nil {
		return nil, fmt.Errorf("get opencode GSD path: %w", err)
	}

	tmpHome, err := os.MkdirTemp("", "kimchi-gsd-*")
	if err != nil {
		return nil, fmt.Errorf("create temp home: %w", err)
	}
	defer os.RemoveAll(tmpHome)

	cmd := exec.Command("npx", args...)
	cmd.Stdin = os.Stdin
	cmd.Env = sandboxedEnv(tmpHome)

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("run gsd-opencode installer: %w", err)
	}

	// gsd-opencode writes to multiple subdirs under ~/.config/opencode
	// (commands/gsd, agents, rules, skills, get-shit-done). Copy the entire
	// opencode tree from the sandbox to the kimchi-managed location.
	srcRoot := filepath.Join(tmpHome, ".config", "opencode")
	if err := copyDir(srcRoot, destPath); err != nil {
		return nil, fmt.Errorf("copy GSD files to kimchi dir: %w", err)
	}

	return &InstallResult{
		Type:      InstallationOpenCode,
		Path:      destPath,
		Installed: []string{"gsd-opencode"},
	}, nil
}

func (i *Installer) installClaudeCode(scope string) (*InstallResult, error) {
	args := []string{"--yes", "get-shit-done-cc@latest", "--claude"}
	if scope == "project" {
		args = append(args, "--local")
	} else {
		args = append(args, "--global")
	}

	destPath, err := getClaudeCodeGSDPath(scope)
	if err != nil {
		return nil, fmt.Errorf("get claude-code GSD path: %w", err)
	}

	tmpHome, err := os.MkdirTemp("", "kimchi-gsd-*")
	if err != nil {
		return nil, fmt.Errorf("create temp home: %w", err)
	}
	defer os.RemoveAll(tmpHome)

	cmd := exec.Command("npx", args...)
	cmd.Stdin = os.Stdin
	cmd.Env = sandboxedEnv(tmpHome)

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("run get-shit-done-cc installer: %w", err)
	}

	// get-shit-done-cc writes to multiple subdirs under ~/.claude
	// (commands/gsd, agents, hooks, get-shit-done). Copy the entire
	// claude tree from the sandbox to the kimchi-managed location.
	srcRoot := filepath.Join(tmpHome, ".claude")
	if err := copyDir(srcRoot, destPath); err != nil {
		return nil, fmt.Errorf("copy GSD files to kimchi dir: %w", err)
	}

	return &InstallResult{
		Type:      InstallationClaudeCode,
		Path:      destPath,
		Installed: []string{"get-shit-done-cc"},
	}, nil
}

func (i *Installer) installCodex(scope string) (*InstallResult, error) {
	args := []string{"--yes", "get-shit-done-cc@latest", "--codex"}
	if scope == "project" {
		args = append(args, "--local")
	} else {
		args = append(args, "--global")
	}

	destPath, err := getCodexGSDPath(scope)
	if err != nil {
		return nil, fmt.Errorf("get codex GSD path: %w", err)
	}

	tmpHome, err := os.MkdirTemp("", "kimchi-gsd-*")
	if err != nil {
		return nil, fmt.Errorf("create temp home: %w", err)
	}
	defer os.RemoveAll(tmpHome)

	cmd := exec.Command("npx", args...)
	cmd.Stdin = os.Stdin
	cmd.Env = sandboxedEnv(tmpHome)

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("run get-shit-done-cc installer: %w", err)
	}

	srcRoot := filepath.Join(tmpHome, ".codex")
	if err := copyDir(srcRoot, destPath); err != nil {
		return nil, fmt.Errorf("copy GSD files to kimchi dir: %w", err)
	}

	return &InstallResult{
		Type:      InstallationCodex,
		Path:      destPath,
		Installed: []string{"get-shit-done-cc"},
	}, nil
}

func getOpenCodeGSDPath(scope string) (string, error) {
	if scope == "project" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(cwd, ".kimchi", "opencode"), nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	// Install directly into the kimchi-managed opencode config dir.
	// The wrapper sets XDG_CONFIG_HOME=~/.config/kimchi, so OpenCode reads
	// from ~/.config/kimchi/opencode/ — no copy step needed at runtime.
	return filepath.Join(homeDir, ".config", "kimchi", "opencode"), nil
}

func getClaudeCodeGSDPath(scope string) (string, error) {
	if scope == "project" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(cwd, ".kimchi", "gsd", "claude-code"), nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "kimchi", "gsd", "claude-code"), nil
}

func getCodexGSDPath(scope string) (string, error) {
	if scope == "project" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(cwd, ".kimchi", "gsd", "codex"), nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "kimchi", "gsd", "codex"), nil
}

func (i *Installer) IsInstalledFor(installType InstallationType, scope string) bool {
	// Check kimchi-managed path.
	var basePath string
	var err error

	switch installType {
	case InstallationOpenCode:
		basePath, err = getOpenCodeGSDPath(scope)
	case InstallationClaudeCode:
		basePath, err = getClaudeCodeGSDPath(scope)
	case InstallationCodex:
		basePath, err = getCodexGSDPath(scope)
	default:
		return false
	}

	if err == nil {
		if info, err := os.Stat(basePath); err == nil && info.IsDir() {
			return true
		}
	}

	// Also check the real tool path (user may have installed GSD directly).
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	var realPaths []string
	switch installType {
	case InstallationOpenCode:
		realPaths = []string{
			filepath.Join(homeDir, ".config", "opencode", "commands", "gsd"),
			filepath.Join(homeDir, ".config", "opencode", "agents"),
		}
	case InstallationClaudeCode:
		realPaths = []string{
			filepath.Join(homeDir, ".claude", "commands", "gsd"),
			filepath.Join(homeDir, ".claude", "agents"),
		}
	case InstallationCodex:
		realPaths = []string{
			filepath.Join(homeDir, ".codex", "agents"),
		}
	}

	for _, p := range realPaths {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return true
		}
	}

	return false
}

// sandboxedEnv returns the current environment with HOME, XDG_CONFIG_HOME, and
// XDG_DATA_HOME redirected to tmpHome. This ensures npm installers write into
// the temp directory regardless of which path convention they follow.
func sandboxedEnv(tmpHome string) []string {
	overrides := map[string]string{
		"HOME":            tmpHome,
		"XDG_CONFIG_HOME": filepath.Join(tmpHome, ".config"),
		"XDG_DATA_HOME":   filepath.Join(tmpHome, ".local", "share"),
	}
	existing := os.Environ()
	merged := make([]string, 0, len(existing)+len(overrides))
	for _, e := range existing {
		key, _, _ := strings.Cut(e, "=")
		if _, ok := overrides[key]; !ok {
			merged = append(merged, e)
		}
	}
	for k, v := range overrides {
		merged = append(merged, k+"="+v)
	}
	return merged
}

// copyDir recursively copies all files from src to dst, creating dst if needed.
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source file: %w", err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return fmt.Errorf("create destination file: %w", err)
	}

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		dstFile.Close()
		return fmt.Errorf("copy file contents: %w", err)
	}

	return dstFile.Close()
}
