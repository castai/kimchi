package update

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	stateTTL  = 24 * time.Hour
	stateFile = "state.json"
	appDir    = "kimchi"
)

// repoState holds the cached update-check result for a single repository.
type repoState struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version"`
	ReleaseURL    string    `json:"release_url,omitempty"`
}

func (s *repoState) IsStale(now time.Time) bool {
	return now.Sub(s.CheckedAt) > stateTTL
}

// state is the top-level structure persisted to disk. It holds cached state
// for multiple repositories, keyed by "owner/name".
type state struct {
	Repos map[string]*repoState `json:"repos"`
}

func repoKey(repo Repo) string {
	return repo.Owner + "/" + repo.Name
}

func statePath() (string, error) {
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appDir, stateFile), nil
}

func loadState() (*state, error) {
	path, err := statePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read state file: %w", err)
	}

	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		// Treat corrupt file as missing state (will trigger re-check).
		return nil, nil
	}

	return &s, nil
}

func saveState(s *state) error {
	path, err := statePath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("write state file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("atomic rename state file: %w", err)
	}

	return nil
}

// loadRepoState returns the cached state for the given repo, or nil if none exists.
func loadRepoState(repo Repo) (*repoState, error) {
	s, err := loadState()
	if err != nil {
		return nil, err
	}
	if s == nil || s.Repos == nil {
		return nil, nil
	}
	return s.Repos[repoKey(repo)], nil
}

// stateMu serializes read-modify-write cycles on the state file so that
// concurrent goroutines (e.g. parallel CLI + harness update checks) don't
// clobber each other's entries.
var stateMu sync.Mutex

// saveRepoState performs a read-modify-write: it loads the full state file,
// updates the entry for the given repo, and writes back the full map.
func saveRepoState(repo Repo, rs *repoState) error {
	stateMu.Lock()
	defer stateMu.Unlock()

	s, err := loadState()
	if err != nil {
		return err
	}
	if s == nil {
		s = &state{}
	}
	if s.Repos == nil {
		s.Repos = make(map[string]*repoState)
	}
	s.Repos[repoKey(repo)] = rs
	return saveState(s)
}
