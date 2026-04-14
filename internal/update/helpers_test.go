package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"
)

func mustVersion(v string) *semver.Version {
	ver, err := semver.NewVersion(v)
	if err != nil {
		panic(err)
	}
	return ver
}

type mockGitHubClient struct {
	latestRelease    *ReleaseInfo
	latestReleaseErr error
	checksum         []byte
	checksumErr      error
	downloadFn       func(ctx context.Context, repo Repo, version, dest string) error
}

func (m *mockGitHubClient) LatestRelease(ctx context.Context, repo Repo) (*ReleaseInfo, error) {
	return m.latestRelease, m.latestReleaseErr
}

func (m *mockGitHubClient) FetchChecksum(ctx context.Context, repo Repo, version string) ([]byte, error) {
	return m.checksum, m.checksumErr
}

func (m *mockGitHubClient) DownloadArchive(ctx context.Context, repo Repo, version, dest string) error {
	if m.downloadFn != nil {
		return m.downloadFn(ctx, repo, version, dest)
	}
	return nil
}

func createTestArchive(t *testing.T, binaryName string, binaryContent []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: binaryName,
		Mode: 0755,
		Size: int64(len(binaryContent)),
	}))
	_, err := tw.Write(binaryContent)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

func sha256sum(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}
