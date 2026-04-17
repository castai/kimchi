package update

import (
	"path/filepath"
	"testing"
)

func TestHarnessDataDir(t *testing.T) {
	t.Run("uses XDG_DATA_HOME when set", func(t *testing.T) {
		xdg := t.TempDir()
		t.Setenv("XDG_DATA_HOME", xdg)

		got := harnessDataDir()
		want := filepath.Join(xdg, "kimchi")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("falls back to HOME/.local/share/kimchi", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		home := t.TempDir()
		t.Setenv("HOME", home)

		got := harnessDataDir()
		want := filepath.Join(home, ".local", "share", "kimchi")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestDataDir(t *testing.T) {
	t.Run("uses XDG_DATA_HOME when set", func(t *testing.T) {
		xdg := t.TempDir()
		t.Setenv("XDG_DATA_HOME", xdg)

		got := dataDir()
		if got != xdg {
			t.Errorf("got %q, want %q", got, xdg)
		}
	})

	t.Run("falls back to HOME/.local/share when XDG_DATA_HOME unset", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		home := t.TempDir()
		t.Setenv("HOME", home)

		got := dataDir()
		want := filepath.Join(home, ".local", "share")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("empty XDG_DATA_HOME treated as unset", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		home := t.TempDir()
		t.Setenv("HOME", home)

		got := dataDir()
		want := filepath.Join(home, ".local", "share")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}
