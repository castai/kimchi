package update

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHarnessDataDir(t *testing.T) {
	t.Run("uses XDG_DATA_HOME when set", func(t *testing.T) {
		xdg := t.TempDir()
		t.Setenv("XDG_DATA_HOME", xdg)

		assert.Equal(t, filepath.Join(xdg, "kimchi"), harnessDataDir())
	})

	t.Run("falls back to HOME/.local/share/kimchi", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		home := t.TempDir()
		t.Setenv("HOME", home)

		assert.Equal(t, filepath.Join(home, ".local", "share", "kimchi"), harnessDataDir())
	})
}

func TestDataDir(t *testing.T) {
	t.Run("uses XDG_DATA_HOME when set", func(t *testing.T) {
		xdg := t.TempDir()
		t.Setenv("XDG_DATA_HOME", xdg)

		assert.Equal(t, xdg, dataDir())
	})

	t.Run("falls back to HOME/.local/share when XDG_DATA_HOME unset", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		home := t.TempDir()
		t.Setenv("HOME", home)

		assert.Equal(t, filepath.Join(home, ".local", "share"), dataDir())
	})

	t.Run("empty XDG_DATA_HOME treated as unset", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		home := t.TempDir()
		t.Setenv("HOME", home)

		assert.Equal(t, filepath.Join(home, ".local", "share"), dataDir())
	})
}
