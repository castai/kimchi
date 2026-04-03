package gsd

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// gsdInstallSpec describes how to install a GSD package into a kimchi-managed
// sandbox directory (used by inject mode).
type gsdInstallSpec struct {
	npxArgs     []string
	tmpSubDir   string // relative to tmp home, e.g. ".config/opencode"
	destPath    func(scope string) (string, error)
	rewriteFile string // config file containing temp paths to rewrite, empty if none
	pkgName     string // npm package name for the result
}

var gsdSpecs = map[InstallationType]gsdInstallSpec{
	InstallationOpenCode: {
		npxArgs:   []string{"--yes", "gsd-opencode@latest", "install", "--global", "--config-dir"},
		tmpSubDir: filepath.Join(".config", "opencode"),
		destPath:  getOpenCodeGSDPath,
		pkgName:   "gsd-opencode",
	},
	InstallationClaudeCode: {
		npxArgs:     []string{"--yes", "get-shit-done-cc@latest", "--claude", "--global", "--config-dir"},
		tmpSubDir:   ".claude",
		destPath:    getClaudeCodeGSDPath,
		rewriteFile: "settings.json",
		pkgName:     "get-shit-done-cc",
	},
	InstallationCodex: {
		npxArgs:     []string{"--yes", "get-shit-done-cc@latest", "--codex", "--global", "--config-dir"},
		tmpSubDir:   ".codex",
		destPath:    getCodexGSDPath,
		rewriteFile: "config.toml",
		pkgName:     "get-shit-done-cc",
	},
}

// Install runs the GSD installer for the given tool. It uses a sandboxed temp
// directory so the user's real config is never modified, then copies the result
// to the kimchi-managed path.
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

	args := make([]string, len(spec.npxArgs)+1)
	copy(args, spec.npxArgs)
	args[len(spec.npxArgs)] = tmpToolDir

	cmd := exec.Command("npx", args...)
	cmd.Env = sandboxedEnv(tmpHome)

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("run %s installer: %w", spec.pkgName, err)
	}

	if err := copyDir(tmpToolDir, destPath); err != nil {
		return nil, fmt.Errorf("copy GSD files to kimchi dir: %w", err)
	}

	if spec.rewriteFile != "" {
		if err := rewritePaths(filepath.Join(destPath, spec.rewriteFile), tmpToolDir, destPath); err != nil {
			return nil, fmt.Errorf("rewrite config paths: %w", err)
		}
	}

	return &InstallResult{
		Type:      installType,
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

// IsInstalledFor reports whether GSD is already installed for the given tool
// and scope, using the same signals the npm packages use internally so that
// our routing (repair vs fresh install) agrees with theirs.
func (i *Installer) IsInstalledFor(installType InstallationType, scope string) bool {
	// Check the kimchi-managed path first.
	if path, err := KimchiManagedPath(installType); err == nil {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return true
		}
	}

	// Then check the real tool path via configRoot.
	root, err := configRoot(installType, scope)
	if err != nil {
		return false
	}

	if installType == InstallationOpenCode {
		// Primary signal used by gsd-opencode: VERSION file.
		versionFile := filepath.Join(root, "get-shit-done", "VERSION")
		if _, err := os.Stat(versionFile); err == nil {
			return true
		}
	}

	// Current (commands/gsd) and legacy (command/gsd) directory structures.
	for _, dir := range gsdCommandDirs(root) {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
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

// sandboxedEnv returns the current environment with home directory variables
// redirected to tmpHome so npm installers write into the temp directory.
func sandboxedEnv(tmpHome string) []string {
	env := map[string]string{
		"HOME":            tmpHome,
		"XDG_CONFIG_HOME": filepath.Join(tmpHome, ".config"),
		"XDG_DATA_HOME":   filepath.Join(tmpHome, ".local", "share"),
	}
	if runtime.GOOS == "windows" {
		env["USERPROFILE"] = tmpHome
		env["APPDATA"] = filepath.Join(tmpHome, "AppData", "Roaming")
		env["LOCALAPPDATA"] = filepath.Join(tmpHome, "AppData", "Local")
	}
	return tools.MergeEnv(env)
}

// copyDir recursively copies all files from src to dst, creating dst if needed.
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip entries that can't be read (e.g. broken symlinks).
			return nil
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
	info, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("stat source file: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		// Resolve the symlink target; skip if broken.
		info, err = os.Stat(src)
		if err != nil {
			return nil // broken symlink — skip silently
		}
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
