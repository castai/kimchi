package claudecode

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/castai/kimchi/internal/config"
	"github.com/castai/kimchi/internal/gsd"
	"github.com/castai/kimchi/internal/tools"
)

// InjectGSD creates symlinks from ~/.claude/ to the kimchi-managed GSD
// directory. Returns the list of symlink paths that were created so they
// can be cleaned up after the process exits.
func InjectGSD() ([]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	kimchiGSD := filepath.Join(homeDir, ".config", "kimchi", "claude-code")
	claudeDir := filepath.Join(homeDir, ".claude")

	var created []string
	if err := symlinkTree(kimchiGSD, claudeDir, &created); err != nil {
		return created, err
	}
	return created, nil
}

// CleanupGSD removes the symlinks created by InjectGSD.
func CleanupGSD(symlinks []string) {
	for _, path := range symlinks {
		// Only remove if it's still a symlink (don't delete real dirs).
		info, err := os.Lstat(path)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			os.Remove(path)
		}
	}
}

// symlinkTree symlinks directories from src into dst. If a target directory
// already exists as a real directory, it recurses into it to symlink children
// instead of skipping the whole tree. Created symlinks are appended to created.
func symlinkTree(src, dst string, created *[]string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read dir %s: %w", src, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		info, err := os.Lstat(dstPath)
		if err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			// Target exists as a real directory — recurse to symlink children.
			if err := symlinkTree(srcPath, dstPath, created); err != nil {
				return err
			}
			continue
		}

		if err := gsd.EnsureSymlink(srcPath, dstPath); err != nil {
			return err
		}
		*created = append(*created, dstPath)
	}

	return nil
}

// Env returns the environment variables needed to run Claude Code
// with Cast AI configuration. Delegates to tools.ClaudeCodeEnvVars.
func Env(apiKey string, telemetryOptIn bool) map[string]string {
	return tools.ClaudeCodeEnvVars(apiKey, telemetryOptIn)
}

// WriteConfig writes the Claude Code settings file for the given scope.
// Delegates to tools.WriteClaudeCodeConfig.
func WriteConfig(scope config.ConfigScope, telemetryOptIn bool) error {
	return tools.WriteClaudeCodeConfig(scope, telemetryOptIn)
}
