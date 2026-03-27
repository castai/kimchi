package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resolveDir(t *testing.T, dir string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return dir
	}
	return resolved
}

func TestExportEnvToShellProfile(t *testing.T) {
	t.Run("appends export to zshrc", func(t *testing.T) {
		tmpDir := resolveDir(t, t.TempDir())
		t.Setenv("HOME", tmpDir)
		t.Setenv("SHELL", "/bin/zsh")

		existing := "# existing config\nalias ll='ls -la'\n"
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".zshrc"), []byte(existing), 0644))

		path, err := ExportEnvToShellProfile("KIMCHI_API_KEY", "test-key-123")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(tmpDir, ".zshrc"), path)

		content, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Contains(t, string(content), "export KIMCHI_API_KEY=test-key-123")
		assert.Contains(t, string(content), "alias ll='ls -la'")
	})

	t.Run("updates existing export with new value", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)
		t.Setenv("SHELL", "/bin/zsh")

		existing := "# config\nexport KIMCHI_API_KEY=old-key\nalias ll='ls -la'\n"
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".zshrc"), []byte(existing), 0644))

		path, err := ExportEnvToShellProfile("KIMCHI_API_KEY", "new-key-456")
		require.NoError(t, err)

		content, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Contains(t, string(content), "export KIMCHI_API_KEY=new-key-456")
		assert.NotContains(t, string(content), "old-key")
		assert.Contains(t, string(content), "alias ll='ls -la'")
	})

	t.Run("does not duplicate on repeated calls", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)
		t.Setenv("SHELL", "/bin/zsh")

		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".zshrc"), []byte(""), 0644))

		_, err := ExportEnvToShellProfile("KIMCHI_API_KEY", "key1")
		require.NoError(t, err)
		_, err = ExportEnvToShellProfile("KIMCHI_API_KEY", "key1")
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(tmpDir, ".zshrc"))
		require.NoError(t, err)

		count := 0
		for _, line := range strings.Split(string(content), "\n") {
			if line == "export KIMCHI_API_KEY=key1" {
				count++
			}
		}
		assert.Equal(t, 1, count, "export line should appear exactly once")
	})


	t.Run("fish shell uses set -gx syntax", func(t *testing.T) {
		tmpDir := resolveDir(t, t.TempDir())
		t.Setenv("HOME", tmpDir)
		t.Setenv("SHELL", "/usr/bin/fish")

		fishDir := filepath.Join(tmpDir, ".config", "fish")
		require.NoError(t, os.MkdirAll(fishDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(fishDir, "config.fish"), []byte("# fish config\n"), 0644))

		path, err := ExportEnvToShellProfile("KIMCHI_API_KEY", "fish-key")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(fishDir, "config.fish"), path)

		content, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Contains(t, string(content), "set -gx KIMCHI_API_KEY fish-key")
	})

	t.Run("creates profile if it does not exist", func(t *testing.T) {
		tmpDir := resolveDir(t, t.TempDir())
		t.Setenv("HOME", tmpDir)
		t.Setenv("SHELL", "/bin/zsh")

		path, err := ExportEnvToShellProfile("KIMCHI_API_KEY", "new-key")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(tmpDir, ".zshrc"), path)

		content, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Contains(t, string(content), "export KIMCHI_API_KEY=new-key")
	})

	t.Run("returns empty path when shell is unknown and no profiles exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)
		t.Setenv("SHELL", "")

		path, err := ExportEnvToShellProfile("KIMCHI_API_KEY", "key")
		require.NoError(t, err)
		assert.Empty(t, path)
	})

	t.Run("follows symlinks", func(t *testing.T) {
		tmpDir := resolveDir(t, t.TempDir())
		t.Setenv("HOME", tmpDir)
		t.Setenv("SHELL", "/bin/zsh")

		// Create real file in a subdirectory (simulating a dotfiles repo)
		realDir := filepath.Join(tmpDir, "dotfiles")
		require.NoError(t, os.MkdirAll(realDir, 0755))
		realFile := filepath.Join(realDir, "zshrc")
		require.NoError(t, os.WriteFile(realFile, []byte("# real config\n"), 0644))

		// Symlink ~/.zshrc -> dotfiles/zshrc
		require.NoError(t, os.Symlink(realFile, filepath.Join(tmpDir, ".zshrc")))

		path, err := ExportEnvToShellProfile("KIMCHI_API_KEY", "sym-key")
		require.NoError(t, err)
		assert.Equal(t, realFile, path)

		// Verify the real file was modified
		content, err := os.ReadFile(realFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "export KIMCHI_API_KEY=sym-key")

		// Verify symlink still exists
		info, err := os.Lstat(filepath.Join(tmpDir, ".zshrc"))
		require.NoError(t, err)
		assert.True(t, info.Mode()&os.ModeSymlink != 0, "symlink should be preserved")
	})

	t.Run("rejects non-UTF-8 content", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)
		t.Setenv("SHELL", "/bin/zsh")

		// Write invalid UTF-8 bytes
		invalid := []byte{0xff, 0xfe, 0x80, 0x81}
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".zshrc"), invalid, 0644))

		path, err := ExportEnvToShellProfile("KIMCHI_API_KEY", "key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "non-UTF-8")
		assert.Empty(t, path)
	})

}

