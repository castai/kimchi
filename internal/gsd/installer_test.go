package gsd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyDir(t *testing.T) {
	t.Run("copies files recursively", func(t *testing.T) {
		src := t.TempDir()
		dst := t.TempDir()

		require.NoError(t, os.MkdirAll(filepath.Join(src, "sub"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(src, "a.txt"), []byte("hello"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("world"), 0644))

		require.NoError(t, copyDir(src, dst))

		data, err := os.ReadFile(filepath.Join(dst, "a.txt"))
		require.NoError(t, err)
		assert.Equal(t, "hello", string(data))

		data, err = os.ReadFile(filepath.Join(dst, "sub", "b.txt"))
		require.NoError(t, err)
		assert.Equal(t, "world", string(data))
	})

	t.Run("creates destination if needed", func(t *testing.T) {
		src := t.TempDir()
		dst := filepath.Join(t.TempDir(), "new", "nested")

		require.NoError(t, os.WriteFile(filepath.Join(src, "file.txt"), []byte("data"), 0644))
		require.NoError(t, copyDir(src, dst))

		data, err := os.ReadFile(filepath.Join(dst, "file.txt"))
		require.NoError(t, err)
		assert.Equal(t, "data", string(data))
	})
}

func TestRewritePaths(t *testing.T) {
	t.Run("replaces old prefix with new", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "config.json")

		require.NoError(t, os.WriteFile(path, []byte(`{"path": "/tmp/old/dir/file"}`), 0644))
		require.NoError(t, rewritePaths(path, "/tmp/old/dir", "/home/user/.config/kimchi"))

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, `{"path": "/home/user/.config/kimchi/file"}`, string(data))
	})

	t.Run("returns nil for missing file", func(t *testing.T) {
		err := rewritePaths("/nonexistent/path", "old", "new")
		assert.NoError(t, err)
	})
}

func TestGetGSDPath(t *testing.T) {
	t.Run("global scope returns kimchi managed path", func(t *testing.T) {
		homeDir, _ := os.UserHomeDir()
		path, err := getGSDPath("opencode", "global")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(homeDir, ".config", "kimchi", "opencode"), path)
	})

	t.Run("project scope returns cwd-relative path", func(t *testing.T) {
		cwd, _ := os.Getwd()
		path, err := getGSDPath("codex", "project")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(cwd, ".kimchi", "codex"), path)
	})
}
