package update

import (
	"context"
	"time"

	"github.com/Masterminds/semver/v3"
)

type CheckResult struct {
	CurrentVersion semver.Version
	LatestVersion  semver.Version
	LatestTag      string
	ReleaseURL     string
}

type CheckOption func(*checkOptions)

type checkOptions struct {
	skipCache bool
}

// WithSkipCache bypasses the local cache and always queries the GitHub API.
func WithSkipCache() CheckOption {
	return func(o *checkOptions) { o.skipCache = true }
}

// Check fetches the latest release version, using a 24h cached state when fresh
// or querying the GitHub API when stale. Returns both current and latest versions
// as parsed semver, letting the caller decide whether an update is needed.
func Check(ctx context.Context, client GitHubClient, currentVersion string, opts ...CheckOption) (*CheckResult, error) {
	var o checkOptions
	for _, opt := range opts {
		opt(&o)
	}
	cur, err := semver.NewVersion(currentVersion)
	if err != nil {
		return nil, err
	}

	var latestTag, releaseURL string

	// Errors loading cache are non-fatal; we fall through to a fresh API call.
	if !o.skipCache {
		state, _ := LoadState()
		if state != nil && !state.IsStale(time.Now()) && state.LatestVersion != "" {
			latestTag = state.LatestVersion
			releaseURL = state.ReleaseURL
		}
	}

	if latestTag == "" {
		info, err := client.LatestRelease(ctx)
		if err != nil {
			return nil, err
		}

		latestTag = info.TagName
		releaseURL = info.HTMLURL

		// Save failure is non-fatal; the next check will query the API again.
		_ = SaveState(&State{
			CheckedAt:     time.Now(),
			LatestVersion: latestTag,
			ReleaseURL:    releaseURL,
		})
	}

	lat, err := semver.NewVersion(latestTag)
	if err != nil {
		return nil, err
	}

	return &CheckResult{
		CurrentVersion: *cur,
		LatestVersion:  *lat,
		LatestTag:      latestTag,
		ReleaseURL:     releaseURL,
	}, nil
}
