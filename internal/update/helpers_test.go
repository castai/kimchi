package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockGitHubClient struct {
	latestRelease    *ReleaseInfo
	latestReleaseErr error
	checksum         []byte
	checksumErr      error
	downloadFn       func(ctx context.Context, version, dest string, progress io.Writer) error
}

func (m *mockGitHubClient) LatestRelease(ctx context.Context) (*ReleaseInfo, error) {
	return m.latestRelease, m.latestReleaseErr
}

func (m *mockGitHubClient) FetchChecksum(ctx context.Context, version string) ([]byte, error) {
	return m.checksum, m.checksumErr
}

func (m *mockGitHubClient) DownloadArchive(ctx context.Context, version, dest string, progress io.Writer) error {
	if m.downloadFn != nil {
		return m.downloadFn(ctx, version, dest, progress)
	}
	return nil
}

func createTestArchive(t *testing.T, binaryContent []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: "kimchi",
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
