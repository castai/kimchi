package update

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowRun_UpdateAvailable_AppliesAndUpdatesCache(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	newBinary := []byte("#!/bin/sh\necho v2.0.0")
	archive := createArchive(t, []archiveFile{{Name: kimchiRepo.Binary, Content: newBinary, Mode: 0755}})
	checksum := sha256sum(archive)

	client := &mockGitHubClient{
		latestRelease: &ReleaseInfo{
			TagName: "v2.0.0",
			HTMLURL: "https://github.com/castai/kimchi/releases/tag/v2.0.0",
		},
		checksum: checksum,
		downloadFn: func(_ context.Context, _ Repo, _, dest string) error {
			return os.WriteFile(dest, archive, 0644)
		},
	}

	targetDir := t.TempDir()
	targetPath := filepath.Join(targetDir, kimchiRepo.Binary)
	require.NoError(t, os.WriteFile(targetPath, []byte("#!/bin/sh\necho v1.0.0"), 0755))

	w := NewWorkflow(kimchiRepo,
		WithClient(client),
		WithCurrentVersionFn(func(_ context.Context) (*semver.Version, error) {
			return mustVersion("1.0.0"), nil
		}),
		WithExecutablePathFn(func() (string, error) { return targetPath, nil }),
	)

	result, err := w.Run(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Updated)
	assert.Equal(t, "1.0.0", result.InstalledVersion.String())
	assert.Equal(t, "2.0.0", result.AvailableVersion.String())

	got, err := os.ReadFile(targetPath)
	require.NoError(t, err)
	assert.Equal(t, newBinary, got)

	rs, err := loadRepoState(kimchiRepo)
	require.NoError(t, err)
	assert.Equal(t, "v2.0.0", rs.LatestVersion)
	assert.False(t, rs.IsStale(time.Now()))
}

func TestWorkflowRun_AlreadyOnLatest_NoUpdateCacheUpdated(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	client := &mockGitHubClient{
		latestRelease: &ReleaseInfo{
			TagName: "v2.0.0",
			HTMLURL: "https://github.com/castai/kimchi/releases/tag/v2.0.0",
		},
	}

	w := NewWorkflow(kimchiRepo,
		WithClient(client),
		WithCurrentVersionFn(func(_ context.Context) (*semver.Version, error) {
			return mustVersion("2.0.0"), nil
		}),
		WithExecutablePathFn(func() (string, error) { return "/unused", nil }),
	)

	result, err := w.Run(context.Background())
	require.NoError(t, err)
	assert.False(t, result.Updated)
	assert.Equal(t, "2.0.0", result.InstalledVersion.String())
	assert.Equal(t, "2.0.0", result.AvailableVersion.String())

	rs, err := loadRepoState(kimchiRepo)
	require.NoError(t, err)
	assert.Equal(t, "v2.0.0", rs.LatestVersion)
}

func TestWorkflowRun_DryRun_NoApplyCacheUpdated(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	client := &mockGitHubClient{
		latestRelease: &ReleaseInfo{
			TagName: "v2.0.0",
			HTMLURL: "https://github.com/castai/kimchi/releases/tag/v2.0.0",
		},
	}

	targetDir := t.TempDir()
	targetPath := filepath.Join(targetDir, kimchiRepo.Binary)
	originalContent := []byte("#!/bin/sh\necho v1.0.0")
	require.NoError(t, os.WriteFile(targetPath, originalContent, 0755))

	w := NewWorkflow(kimchiRepo,
		WithClient(client),
		WithDryRun(),
		WithCurrentVersionFn(func(_ context.Context) (*semver.Version, error) {
			return mustVersion("1.0.0"), nil
		}),
		WithExecutablePathFn(func() (string, error) { return targetPath, nil }),
	)

	result, err := w.Run(context.Background())
	require.NoError(t, err)
	assert.False(t, result.Updated)
	assert.Equal(t, "2.0.0", result.AvailableVersion.String())

	// Binary must not have been replaced.
	got, err := os.ReadFile(targetPath)
	require.NoError(t, err)
	assert.Equal(t, originalContent, got)

	// Cache should still be updated by the check step.
	rs, err := loadRepoState(kimchiRepo)
	require.NoError(t, err)
	assert.Equal(t, "v2.0.0", rs.LatestVersion)
}

func TestWorkflowRun_UserDeclines_NoUpdate(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	client := &mockGitHubClient{
		latestRelease: &ReleaseInfo{
			TagName: "v2.0.0",
			HTMLURL: "https://github.com/castai/kimchi/releases/tag/v2.0.0",
		},
	}

	targetDir := t.TempDir()
	targetPath := filepath.Join(targetDir, kimchiRepo.Binary)
	originalContent := []byte("#!/bin/sh\necho v1.0.0")
	require.NoError(t, os.WriteFile(targetPath, originalContent, 0755))

	w := NewWorkflow(kimchiRepo,
		WithClient(client),
		WithCurrentVersionFn(func(_ context.Context) (*semver.Version, error) {
			return mustVersion("1.0.0"), nil
		}),
		WithExecutablePathFn(func() (string, error) { return targetPath, nil }),
		WithConfirmFn(func(_, _ *semver.Version) (bool, error) { return false, nil }),
	)

	result, err := w.Run(context.Background())
	require.NoError(t, err)
	assert.False(t, result.Updated)

	// Binary must not have been replaced.
	got, err := os.ReadFile(targetPath)
	require.NoError(t, err)
	assert.Equal(t, originalContent, got)

	// Cache should still be updated by the check step.
	rs, err := loadRepoState(kimchiRepo)
	require.NoError(t, err)
	assert.Equal(t, "v2.0.0", rs.LatestVersion)
}

func TestWorkflowRun_FreshCache_NoAPICall(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	// Pre-populate cache with a fresh entry.
	require.NoError(t, saveRepoState(kimchiRepo, &repoState{
		CheckedAt:     time.Now(),
		LatestVersion: "v2.0.0",
		ReleaseURL:    "https://github.com/castai/kimchi/releases/tag/v2.0.0",
	}))

	// Client would fail if called — proves cache is used.
	client := &mockGitHubClient{
		latestReleaseErr: assert.AnError,
	}

	w := NewWorkflow(kimchiRepo,
		WithClient(client),
		WithCurrentVersionFn(func(_ context.Context) (*semver.Version, error) {
			return mustVersion("2.0.0"), nil
		}),
		WithExecutablePathFn(func() (string, error) { return "/unused", nil }),
	)

	result, err := w.Run(context.Background())
	require.NoError(t, err)
	assert.False(t, result.Updated)
	assert.Equal(t, "2.0.0", result.AvailableVersion.String())
}

func TestWorkflowRun_StaleCache_RefetchesAndUpdates(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	// Pre-populate cache with a stale entry.
	require.NoError(t, saveRepoState(kimchiRepo, &repoState{
		CheckedAt:     time.Now().Add(-stateTTL - time.Hour),
		LatestVersion: "v1.0.0",
		ReleaseURL:    "https://github.com/castai/kimchi/releases/tag/v1.0.0",
	}))

	newBinary := []byte("#!/bin/sh\necho v2.0.0")
	archive := createArchive(t, []archiveFile{{Name: kimchiRepo.Binary, Content: newBinary, Mode: 0755}})
	checksum := sha256sum(archive)

	client := &mockGitHubClient{
		latestRelease: &ReleaseInfo{
			TagName: "v2.0.0",
			HTMLURL: "https://github.com/castai/kimchi/releases/tag/v2.0.0",
		},
		checksum: checksum,
		downloadFn: func(_ context.Context, _ Repo, _, dest string) error {
			return os.WriteFile(dest, archive, 0644)
		},
	}

	targetDir := t.TempDir()
	targetPath := filepath.Join(targetDir, kimchiRepo.Binary)
	require.NoError(t, os.WriteFile(targetPath, []byte("#!/bin/sh\necho v1.0.0"), 0755))

	w := NewWorkflow(kimchiRepo,
		WithClient(client),
		WithCurrentVersionFn(func(_ context.Context) (*semver.Version, error) {
			return mustVersion("1.0.0"), nil
		}),
		WithExecutablePathFn(func() (string, error) { return targetPath, nil }),
	)

	result, err := w.Run(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Updated)

	rs, err := loadRepoState(kimchiRepo)
	require.NoError(t, err)
	assert.Equal(t, "v2.0.0", rs.LatestVersion)
	assert.False(t, rs.IsStale(time.Now()))
}

func TestWorkflowRun_ApplyFailure_OriginalPreserved(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	client := &mockGitHubClient{
		latestRelease: &ReleaseInfo{
			TagName: "v2.0.0",
			HTMLURL: "https://github.com/castai/kimchi/releases/tag/v2.0.0",
		},
		checksumErr: assert.AnError, // FetchChecksum will fail.
	}

	targetDir := t.TempDir()
	targetPath := filepath.Join(targetDir, kimchiRepo.Binary)
	originalContent := []byte("#!/bin/sh\necho v1.0.0")
	require.NoError(t, os.WriteFile(targetPath, originalContent, 0755))

	w := NewWorkflow(kimchiRepo,
		WithClient(client),
		WithCurrentVersionFn(func(_ context.Context) (*semver.Version, error) {
			return mustVersion("1.0.0"), nil
		}),
		WithExecutablePathFn(func() (string, error) { return targetPath, nil }),
	)

	_, err := w.Run(context.Background())
	assert.Error(t, err)
	assert.ErrorContains(t, err, "fetch checksum")

	// Original binary must still be intact.
	got, err := os.ReadFile(targetPath)
	require.NoError(t, err)
	assert.Equal(t, originalContent, got)
}

func TestWorkflowRun_FreshInstall_InstallsAndUpdatesCache(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	newBinary := []byte("#!/bin/sh\necho v1.0.0")
	archive := createArchive(t, []archiveFile{{Name: kimchiDevRepo.Binary, Content: newBinary, Mode: 0755}})
	checksum := sha256sum(archive)

	client := &mockGitHubClient{
		latestRelease: &ReleaseInfo{
			TagName: "v1.0.0",
			HTMLURL: "https://github.com/castai/kimchi-dev/releases/tag/v1.0.0",
		},
		checksum: checksum,
		downloadFn: func(_ context.Context, _ Repo, _, dest string) error {
			return os.WriteFile(dest, archive, 0644)
		},
	}

	targetDir := t.TempDir()
	targetPath := filepath.Join(targetDir, kimchiDevRepo.Binary)
	// Target does NOT exist — fresh install.

	w := NewWorkflow(kimchiDevRepo,
		WithClient(client),
		WithCurrentVersionFn(func(_ context.Context) (*semver.Version, error) {
			return nil, nil // not installed
		}),
		WithExecutablePathFn(func() (string, error) { return targetPath, nil }),
	)

	result, err := w.Run(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Updated)
	assert.Nil(t, result.InstalledVersion)
	assert.Equal(t, "1.0.0", result.AvailableVersion.String())

	got, err := os.ReadFile(targetPath)
	require.NoError(t, err)
	assert.Equal(t, newBinary, got)

	rs, err := loadRepoState(kimchiDevRepo)
	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", rs.LatestVersion)
	assert.False(t, rs.IsStale(time.Now()))
}

func TestCheckCLIUpdate_MapsFieldsCorrectly(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	client := &mockGitHubClient{
		latestRelease: &ReleaseInfo{
			TagName: "v2.0.0",
			HTMLURL: "https://github.com/castai/kimchi/releases/tag/v2.0.0",
		},
	}

	status, err := CheckCLIUpdate(context.Background(),
		WithClient(client),
		WithCurrentVersionFn(func(_ context.Context) (*semver.Version, error) {
			return mustVersion("1.0.0"), nil
		}),
	)
	require.NoError(t, err)

	// CurrentVersion should be the installed version (1.0.0), not the latest.
	assert.Equal(t, "1.0.0", status.CurrentVersion.String())
	// LatestVersion should be the available release (2.0.0).
	assert.Equal(t, "2.0.0", status.LatestVersion.String())
	assert.True(t, status.HasUpdate)
	assert.True(t, status.Installed())
	assert.Equal(t, "kimchi", status.DisplayName)
}

func TestCheckHarnessUpdate_FreshInstall(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	client := &mockGitHubClient{
		latestRelease: &ReleaseInfo{
			TagName: "v1.0.0",
			HTMLURL: "https://github.com/castai/kimchi-dev/releases/tag/v1.0.0",
		},
	}

	status, err := CheckHarnessUpdate(context.Background(),
		WithClient(client),
		WithCurrentVersionFn(func(_ context.Context) (*semver.Version, error) {
			return nil, nil // not installed
		}),
	)
	require.NoError(t, err)

	assert.Nil(t, status.CurrentVersion)
	assert.Equal(t, "1.0.0", status.LatestVersion.String())
	assert.True(t, status.HasUpdate)
	assert.False(t, status.Installed())
	assert.Equal(t, "coding harness", status.DisplayName)
}

func TestWorkflowRun_VerificationFails_RollsBackUpdate(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	// The new binary will fail verification (exits non-zero).
	brokenBinary := []byte("#!/bin/sh\nexit 1")
	archive := createArchive(t, []archiveFile{{Name: kimchiRepo.Binary, Content: brokenBinary, Mode: 0755}})
	checksum := sha256sum(archive)

	client := &mockGitHubClient{
		latestRelease: &ReleaseInfo{
			TagName: "v2.0.0",
			HTMLURL: "https://github.com/castai/kimchi/releases/tag/v2.0.0",
		},
		checksum: checksum,
		downloadFn: func(_ context.Context, _ Repo, _, dest string) error {
			return os.WriteFile(dest, archive, 0644)
		},
	}

	targetDir := t.TempDir()
	targetPath := filepath.Join(targetDir, kimchiRepo.Binary)
	originalContent := []byte("#!/bin/sh\necho v1.0.0")
	require.NoError(t, os.WriteFile(targetPath, originalContent, 0755))

	w := NewWorkflow(kimchiRepo,
		WithClient(client),
		WithCurrentVersionFn(func(_ context.Context) (*semver.Version, error) {
			return mustVersion("1.0.0"), nil
		}),
		WithExecutablePathFn(func() (string, error) { return targetPath, nil }),
	)

	_, err := w.Run(context.Background())
	assert.Error(t, err)
	assert.ErrorContains(t, err, "binary verification failed")

	// Original binary must be restored.
	got, err := os.ReadFile(targetPath)
	require.NoError(t, err)
	assert.Equal(t, originalContent, got)
}

func TestNewWorkflow_DefaultClient_IsUsable(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	// Create a workflow without WithClient — should get a working default.
	w := NewWorkflow(kimchiRepo,
		WithCurrentVersionFn(func(_ context.Context) (*semver.Version, error) {
			return mustVersion("1.0.0"), nil
		}),
	)

	// The client field should be non-nil and properly initialized.
	// We can't call Run() without a real server, but we can verify
	// it doesn't panic when the client is used.
	assert.NotNil(t, w.client)
}
