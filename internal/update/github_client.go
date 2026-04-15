package update

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
)

const (
	defaultGitHubAPIBase = "https://api.github.com"
	defaultGitHubBase    = "https://github.com"
)

// Repo identifies a GitHub repository and the binary it produces.
type Repo struct {
	Owner  string
	Name   string
	Binary string
}

var (
	kimchiRepo    = Repo{Owner: "castai", Name: "kimchi", Binary: "kimchi"}
	kimchiDevRepo = Repo{Owner: "castai", Name: "kimchi-dev", Binary: "kimchi-code"}
)

type ReleaseInfo struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// GitHubClient is the interface used by Check and Apply to interact with
// the GitHub releases API.
type GitHubClient interface {
	LatestRelease(ctx context.Context, repo Repo) (*ReleaseInfo, error)
	FetchChecksum(ctx context.Context, repo Repo, version string) ([]byte, error)
	DownloadArchive(ctx context.Context, repo Repo, version, dest string) error
}

type githubClient struct {
	httpClient    *http.Client
	githubAPIBase string
	githubBase    string
}

type GitHubClientOption func(*githubClient)

func WithHTTPClient(c *http.Client) GitHubClientOption {
	return func(g *githubClient) { g.httpClient = c }
}

func WithGitHubAPIBase(url string) GitHubClientOption {
	return func(g *githubClient) { g.githubAPIBase = url }
}

func WithGitHubBase(url string) GitHubClientOption {
	return func(g *githubClient) { g.githubBase = url }
}

func NewGitHubClient(opts ...GitHubClientOption) GitHubClient {
	g := &githubClient{
		httpClient:    http.DefaultClient,
		githubAPIBase: defaultGitHubAPIBase,
		githubBase:    defaultGitHubBase,
	}
	for _, o := range opts {
		o(g)
	}
	return g
}

func (g *githubClient) LatestRelease(ctx context.Context, repo Repo) (*ReleaseInfo, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", g.githubAPIBase, repo.Owner, repo.Name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch latest release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github API returned %d", resp.StatusCode)
	}

	var info ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &info, nil
}

func assetName(repo Repo) string {
	return fmt.Sprintf("%s_%s_%s.tar.gz", repo.Binary, runtime.GOOS, runtime.GOARCH)
}

// FetchChecksum downloads checksums.txt for the given version tag and returns
// the SHA256 hash for the current platform's asset as raw bytes.
func (g *githubClient) FetchChecksum(ctx context.Context, repo Repo, version string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s/%s/releases/download/%s/checksums.txt", g.githubBase, repo.Owner, repo.Name, version)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create checksum request: %w", err)
	}
	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download checksums: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksums download returned %d", resp.StatusCode)
	}

	target := assetName(repo)
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) == 2 && parts[1] == target {
			hash, err := hex.DecodeString(parts[0])
			if err != nil {
				return nil, fmt.Errorf("decode checksum hex: %w", err)
			}
			return hash, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read checksums: %w", err)
	}

	return nil, fmt.Errorf("checksum not found for %s", target)
}

// DownloadArchive downloads the release archive to the given dest path.
func (g *githubClient) DownloadArchive(ctx context.Context, repo Repo, version, dest string) error {
	url := fmt.Sprintf("%s/%s/%s/releases/download/%s/%s", g.githubBase, repo.Owner, repo.Name, version, assetName(repo))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	resp, err := g.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("release download returned %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create file %s: %w", dest, err)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		return fmt.Errorf("write archive to disk: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close archive file: %w", err)
	}
	return nil
}
