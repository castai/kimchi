package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubClient_LatestRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
		_, _ = w.Write([]byte(`{"tag_name":"v1.5.0","html_url":"https://github.com/castai/kimchi/releases/tag/v1.5.0"}`))
	}))
	defer srv.Close()

	client := NewGitHubClient(WithGitHubAPIBase(srv.URL))
	info, err := client.LatestRelease(context.Background(), kimchiRepo)
	require.NoError(t, err)
	assert.Equal(t, "v1.5.0", info.TagName)
	assert.Equal(t, "https://github.com/castai/kimchi/releases/tag/v1.5.0", info.HTMLURL)
}

func TestGitHubClient_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	client := NewGitHubClient(WithGitHubAPIBase(srv.URL))
	_, err := client.LatestRelease(context.Background(), kimchiRepo)
	assert.Error(t, err)
}

func TestGitHubClient_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{invalid`))
	}))
	defer srv.Close()

	client := NewGitHubClient(WithGitHubAPIBase(srv.URL))
	_, err := client.LatestRelease(context.Background(), kimchiRepo)
	assert.Error(t, err)
}

func TestGitHubClient_FetchChecksum(t *testing.T) {
	archive := createTestArchive(t, kimchiRepo.Binary, []byte("binary"))
	hash := sha256.Sum256(archive)
	hashHex := hex.EncodeToString(hash[:])
	asset := fmt.Sprintf("%s_%s_%s.tar.gz", kimchiRepo.Binary, runtime.GOOS, runtime.GOARCH)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Respond with multiple entries to verify correct line is matched.
		_, _ = fmt.Fprintf(w, "abc123def456  other_file.tar.gz\n%s  %s\n", hashHex, asset)
	}))
	defer srv.Close()

	client := NewGitHubClient(WithGitHubBase(srv.URL))
	checksum, err := client.FetchChecksum(context.Background(), kimchiRepo, "v1.0.0")
	require.NoError(t, err)
	assert.Equal(t, hash[:], checksum)
}

func TestGitHubClient_DownloadArchive(t *testing.T) {
	content := []byte("archive-content")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	defer srv.Close()

	client := NewGitHubClient(WithGitHubBase(srv.URL))
	dest := filepath.Join(t.TempDir(), "download.tar.gz")
	require.NoError(t, client.DownloadArchive(context.Background(), kimchiRepo, "v1.0.0", dest))

	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestGitHubClient_FetchChecksum_AssetNotInList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "abc123  some_other_asset.tar.gz\n")
	}))
	defer srv.Close()

	client := NewGitHubClient(WithGitHubBase(srv.URL))
	_, err := client.FetchChecksum(context.Background(), kimchiRepo, "v1.0.0")
	assert.ErrorContains(t, err, "checksum not found")
}
