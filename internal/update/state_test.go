package update

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoState_IsStale(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		checkedAt time.Time
		want      bool
	}{
		{"zero value", time.Time{}, true},
		{"expired", now.Add(-stateTTL - time.Second), true},
		{"fresh", now.Add(-23 * time.Hour), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &repoState{CheckedAt: tt.checkedAt}
			assert.Equal(t, tt.want, s.IsStale(now))
		})
	}
}

func TestLoadRepoState_MissingFile(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	got, err := loadRepoState(kimchiRepo)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestLoadRepoState_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	stateDir := filepath.Join(dir, appDir)
	require.NoError(t, os.MkdirAll(stateDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, stateFile), []byte("{invalid"), 0600))

	got, err := loadRepoState(kimchiRepo)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestSaveAndLoadRepoState_RoundTrip(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	want := &repoState{
		CheckedAt:     time.Now().Truncate(time.Second),
		LatestVersion: "v1.2.3",
	}
	require.NoError(t, saveRepoState(kimchiRepo, want))

	got, err := loadRepoState(kimchiRepo)
	require.NoError(t, err)
	assert.Empty(t, cmp.Diff(want, got))
}

func TestSaveRepoState_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	require.NoError(t, saveRepoState(kimchiRepo, &repoState{LatestVersion: "v1.0.0"}))

	info, err := os.Stat(filepath.Join(dir, appDir, stateFile))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestSaveRepoState_MultiRepoIsolation(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	kimchiState := &repoState{
		CheckedAt:     time.Now().Truncate(time.Second),
		LatestVersion: "v1.0.0",
	}
	devState := &repoState{
		CheckedAt:     time.Now().Truncate(time.Second),
		LatestVersion: "v0.5.0",
	}

	require.NoError(t, saveRepoState(kimchiRepo, kimchiState))
	require.NoError(t, saveRepoState(kimchiDevRepo, devState))

	gotKimchi, err := loadRepoState(kimchiRepo)
	require.NoError(t, err)
	assert.Empty(t, cmp.Diff(kimchiState, gotKimchi))

	gotDev, err := loadRepoState(kimchiDevRepo)
	require.NoError(t, err)
	assert.Empty(t, cmp.Diff(devState, gotDev))
}
