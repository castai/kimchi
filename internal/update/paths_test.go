package update

import (
	"path/filepath"
	"testing"
)

func TestDataDir(t *testing.T) {
	t.Run("uses XDG_DATA_HOME when set", func(t *testing.T) {
		xdgDataHome := t.TempDir()
		t.Setenv("XDG_DATA_HOME", xdgDataHome)

		got, err := dataDir()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != xdgDataHome {
			t.Errorf("got %q, want %q", got, xdgDataHome)
		}
	})

	t.Run("falls back to HOME/.local/share when XDG_DATA_HOME unset", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		home := t.TempDir()
		t.Setenv("HOME", home)

		got, err := dataDir()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join(home, ".local", "share")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("empty XDG_DATA_HOME treated as unset", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		home := t.TempDir()
		t.Setenv("HOME", home)

		got, err := dataDir()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join(home, ".local", "share")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}
