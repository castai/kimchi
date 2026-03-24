package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadAndVerify_Success(t *testing.T) {
	binaryContent := []byte("#!/bin/sh\necho kimchi")
	archive := createTestArchive(t, binaryContent)
	checksum := sha256sum(archive)

	client := &mockGitHubClient{
		downloadFn: func(_ context.Context, _, dest string, _ io.Writer) error {
			return os.WriteFile(dest, archive, 0644)
		},
	}

	binaryPath, err := downloadAndVerify(context.Background(), client, "v1.0.0", checksum, nil)
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(filepath.Dir(binaryPath)) }()

	got, err := os.ReadFile(binaryPath)
	require.NoError(t, err)
	assert.Empty(t, cmp.Diff(binaryContent, got))
}

func TestDownloadAndVerify_ChecksumMismatch(t *testing.T) {
	archive := createTestArchive(t, []byte("binary"))
	badChecksum := make([]byte, 32) // all zeros

	client := &mockGitHubClient{
		downloadFn: func(_ context.Context, _, dest string, _ io.Writer) error {
			return os.WriteFile(dest, archive, 0644)
		},
	}

	_, err := downloadAndVerify(context.Background(), client, "v1.0.0", badChecksum, nil)
	assert.ErrorContains(t, err, "checksum mismatch")
}

func TestDownloadAndVerify_DownloadError(t *testing.T) {
	client := &mockGitHubClient{
		downloadFn: func(_ context.Context, _, _ string, _ io.Writer) error {
			return assert.AnError
		},
	}

	_, err := downloadAndVerify(context.Background(), client, "v1.0.0", make([]byte, 32), nil)
	assert.ErrorContains(t, err, "download archive")
}

func TestApply_Success(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	// Create a fake "new" binary: a shell script that responds to "version".
	newBinary := []byte("#!/bin/sh\necho v2.0.0")
	archive := createTestArchive(t, newBinary)
	checksum := sha256sum(archive)

	client := &mockGitHubClient{
		checksum: checksum,
		downloadFn: func(_ context.Context, _, dest string, _ io.Writer) error {
			return os.WriteFile(dest, archive, 0644)
		},
	}

	// Create a fake "current" binary to be replaced.
	targetDir := t.TempDir()
	targetPath := filepath.Join(targetDir, "kimchi")
	require.NoError(t, os.WriteFile(targetPath, []byte("#!/bin/sh\necho v1.0.0"), 0755))

	err := Apply(context.Background(), client, "v2.0.0", WithExecutablePath(targetPath))
	require.NoError(t, err)

	// Verify the binary was replaced.
	got, err := os.ReadFile(targetPath)
	require.NoError(t, err)
	assert.Equal(t, newBinary, got)
}

func TestApply_ChecksumFetchError(t *testing.T) {
	client := &mockGitHubClient{
		checksumErr: assert.AnError,
	}

	err := Apply(context.Background(), client, "v2.0.0")
	assert.ErrorContains(t, err, "fetch checksum")
}

func TestApply_RollsBack_WhenVerificationFails(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	// Create a "new" binary that will fail verification (exits non-zero).
	badBinary := []byte("#!/bin/sh\nexit 1")
	archive := createTestArchive(t, badBinary)
	checksum := sha256sum(archive)

	client := &mockGitHubClient{
		checksum: checksum,
		downloadFn: func(_ context.Context, _, dest string, _ io.Writer) error {
			return os.WriteFile(dest, archive, 0644)
		},
	}

	// Create a fake "current" binary.
	targetDir := t.TempDir()
	targetPath := filepath.Join(targetDir, "kimchi")
	originalContent := []byte("#!/bin/sh\necho v1.0.0")
	require.NoError(t, os.WriteFile(targetPath, originalContent, 0755))

	err := Apply(context.Background(), client, "v2.0.0", WithExecutablePath(targetPath))
	assert.ErrorContains(t, err, "verification failed")

	// Verify the original binary was restored.
	got, err := os.ReadFile(targetPath)
	require.NoError(t, err)
	assert.Equal(t, originalContent, got)
}

func TestExtractBinary_NoBinaryInArchive(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "not-kimchi", Mode: 0755, Size: 5}))
	_, _ = tw.Write([]byte("hello"))
	_ = tw.Close()
	_ = gw.Close()
	_, err := extractBinary(&buf)
	assert.ErrorContains(t, err, "not found")
}
