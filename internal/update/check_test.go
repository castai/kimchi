package update

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheck_FetchesFromAPI_WhenNoCache(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	client := &mockGitHubClient{
		latestRelease: &ReleaseInfo{
			TagName: "v1.5.0",
			HTMLURL: "https://github.com/castai/kimchi/releases/tag/v1.5.0",
		},
	}

	res, err := Check(context.Background(), client, "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", res.CurrentVersion.String())
	assert.Equal(t, "1.5.0", res.LatestVersion.String())
	assert.Equal(t, "https://github.com/castai/kimchi/releases/tag/v1.5.0", res.ReleaseURL)
}

func TestCheck_UsesCache_WhenFresh(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	// Write a fresh cached state.
	state := &State{
		CheckedAt:     time.Now(),
		LatestVersion: "v2.0.0",
		ReleaseURL:    "https://github.com/castai/kimchi/releases/tag/v2.0.0",
	}
	stateDir := filepath.Join(dir, appDir)
	require.NoError(t, os.MkdirAll(stateDir, 0700))
	data, err := json.MarshalIndent(state, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, stateFile), data, 0600))

	// Client should NOT be called — pass one that would fail.
	client := &mockGitHubClient{
		latestReleaseErr: assert.AnError,
	}

	res, err := Check(context.Background(), client, "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", res.LatestVersion.String())
	assert.Equal(t, "https://github.com/castai/kimchi/releases/tag/v2.0.0", res.ReleaseURL)
}

func TestCheck_FetchesFromAPI_WhenCacheStale(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	// Write a stale cached state.
	state := &State{
		CheckedAt:     time.Now().Add(-stateTTL - time.Hour),
		LatestVersion: "v1.1.0",
		ReleaseURL:    "https://github.com/castai/kimchi/releases/tag/v1.1.0",
	}
	stateDir := filepath.Join(dir, appDir)
	require.NoError(t, os.MkdirAll(stateDir, 0700))
	data, err := json.MarshalIndent(state, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, stateFile), data, 0600))

	client := &mockGitHubClient{
		latestRelease: &ReleaseInfo{
			TagName: "v1.8.0",
			HTMLURL: "https://github.com/castai/kimchi/releases/tag/v1.8.0",
		},
	}

	res, err := Check(context.Background(), client, "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "1.8.0", res.LatestVersion.String())
	assert.Equal(t, "https://github.com/castai/kimchi/releases/tag/v1.8.0", res.ReleaseURL)
}

func TestCheck_SavesStateAfterAPICall(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	client := &mockGitHubClient{
		latestRelease: &ReleaseInfo{
			TagName: "v1.5.0",
			HTMLURL: "https://github.com/castai/kimchi/releases/tag/v1.5.0",
		},
	}

	_, err := Check(context.Background(), client, "1.0.0")
	require.NoError(t, err)

	got, err := LoadState()
	require.NoError(t, err)
	assert.Equal(t, "v1.5.0", got.LatestVersion)
	assert.Equal(t, "https://github.com/castai/kimchi/releases/tag/v1.5.0", got.ReleaseURL)
	assert.False(t, got.IsStale(time.Now()))
}

func TestCheck_ReturnsError_WhenAPIFails(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	client := &mockGitHubClient{
		latestReleaseErr: assert.AnError,
	}

	_, err := Check(context.Background(), client, "1.0.0")
	assert.Error(t, err)
}

func TestCheck_ReturnsError_WhenLatestTagInvalid(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	client := &mockGitHubClient{
		latestRelease: &ReleaseInfo{
			TagName: "not-semver",
		},
	}

	_, err := Check(context.Background(), client, "1.0.0")
	assert.Error(t, err)
}
