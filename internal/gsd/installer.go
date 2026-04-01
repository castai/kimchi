package gsd

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/castai/kimchi/internal/tools"
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

// gsdInstallSpec describes how to install a GSD package for a specific tool.
type gsdInstallSpec struct {
	installType InstallationType
	npxArgs     []string
	tmpSubDir   string // relative to tmp home, e.g. ".config/opencode"
	destPath    func(scope string) (string, error)
	rewriteFile string // config file containing temp paths to rewrite, empty if none
	pkgName     string // npm package name for the result
}

var gsdSpecs = map[InstallationType]gsdInstallSpec{
	InstallationOpenCode: {
		installType: InstallationOpenCode,
		npxArgs:     []string{"--yes", "gsd-opencode@latest", "install", "--global", "--config-dir"},
		tmpSubDir:   filepath.Join(".config", "opencode"),
		destPath:    getOpenCodeGSDPath,
		pkgName:     "gsd-opencode",
	},
	InstallationClaudeCode: {
		installType: InstallationClaudeCode,
		npxArgs:     []string{"--yes", "get-shit-done-cc@latest", "--claude", "--global", "--config-dir"},
		tmpSubDir:   ".claude",
		destPath:    getClaudeCodeGSDPath,
		rewriteFile: "settings.json",
		pkgName:     "get-shit-done-cc",
	},
	InstallationCodex: {
		installType: InstallationCodex,
		npxArgs:     []string{"--yes", "get-shit-done-cc@latest", "--codex", "--global", "--config-dir"},
		tmpSubDir:   ".codex",
		destPath:    getCodexGSDPath,
		rewriteFile: "config.toml",
		pkgName:     "get-shit-done-cc",
	},
}

func (i *Installer) Install(installType InstallationType, scope string) (*InstallResult, error) {
	spec, ok := gsdSpecs[installType]
	if !ok {
		return nil, fmt.Errorf("unsupported tool for GSD: %s", installType)
	}

	destPath, err := spec.destPath(scope)
	if err != nil {
		return nil, fmt.Errorf("get %s GSD path: %w", installType, err)
	}

	tmpHome, err := os.MkdirTemp("", "kimchi-gsd-*")
	if err != nil {
		return nil, fmt.Errorf("create temp home: %w", err)
	}
	defer os.RemoveAll(tmpHome)

	tmpToolDir := filepath.Join(tmpHome, spec.tmpSubDir)

	// Append the temp tool dir as the --config-dir value.
	args := append(spec.npxArgs, tmpToolDir)

	cmd := exec.Command("npx", args...)
	cmd.Env = sandboxedEnv(tmpHome)

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("run %s installer: %w", spec.pkgName, err)
	}

	if err := copyDir(tmpToolDir, destPath); err != nil {
		return nil, fmt.Errorf("copy GSD files to kimchi dir: %w", err)
	}

	// Some installers write absolute temp paths into config files.
	if spec.rewriteFile != "" {
		if err := rewritePaths(filepath.Join(destPath, spec.rewriteFile), tmpToolDir, destPath); err != nil {
			return nil, fmt.Errorf("rewrite config paths: %w", err)
		}
	}

	return &InstallResult{
		Type:      spec.installType,
		Path:      destPath,
		Installed: []string{spec.pkgName},
	}, nil
}

func getGSDPath(toolName, scope string) (string, error) {
	if scope == "project" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(cwd, ".kimchi", toolName), nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "kimchi", toolName), nil
}

func getOpenCodeGSDPath(scope string) (string, error)   { return getGSDPath("opencode", scope) }
func getClaudeCodeGSDPath(scope string) (string, error) { return getGSDPath("claude-code", scope) }
func getCodexGSDPath(scope string) (string, error)      { return getGSDPath("codex", scope) }

func (i *Installer) IsInstalledFor(installType InstallationType, scope string) bool {
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
			filepath.Join(homeDir, ".config", "opencode", "get-shit-done"),
			filepath.Join(homeDir, ".config", "opencode", "commands", "gsd"),
		}
	case InstallationClaudeCode:
		realPaths = []string{
			filepath.Join(homeDir, ".claude", "get-shit-done"),
			filepath.Join(homeDir, ".claude", "commands", "gsd"),
		}
	case InstallationCodex:
		realPaths = []string{
			filepath.Join(homeDir, ".codex", "get-shit-done"),
		}
	}

	for _, p := range realPaths {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return true
		}
	}

	return false
}

// rewritePaths reads a file, replaces all occurrences of oldPrefix with
// newPrefix, and writes it back. Used to fix absolute paths that GSD installers
// bake into config files pointing to the temp sandbox directory.
func rewritePaths(filePath, oldPrefix, newPrefix string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	updated := strings.ReplaceAll(string(data), oldPrefix, newPrefix)
	return os.WriteFile(filePath, []byte(updated), 0644)
}

// sandboxedEnv returns the current environment with HOME, XDG_CONFIG_HOME, and
// XDG_DATA_HOME redirected to tmpHome. This ensures npm installers write into
// the temp directory regardless of which path convention they follow.
func sandboxedEnv(tmpHome string) []string {
	return tools.MergeEnv(map[string]string{
		"HOME":            tmpHome,
		"XDG_CONFIG_HOME": filepath.Join(tmpHome, ".config"),
		"XDG_DATA_HOME":   filepath.Join(tmpHome, ".local", "share"),
	})
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
