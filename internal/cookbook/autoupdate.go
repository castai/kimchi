package cookbook

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	// defaultAutoUpdateSecs is 24 hours — cookbooks don't change as frequently
	// as Homebrew formulae, so a daily pull is plenty.
	defaultAutoUpdateSecs = 24 * 60 * 60
)

func autoUpdateStampPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kimchi", "last_cookbook_update"), nil
}

// AutoUpdateIfStale pulls all registered cookbooks if the auto-update interval
// has elapsed since the last pull. It is a no-op when:
//   - KIMCHI_NO_AUTO_UPDATE is set to any non-empty value
//   - No cookbooks are registered
//   - The stamp file is fresh enough
//
// Errors from individual cookbook pulls are written to errW and do not abort
// the update of other cookbooks (mirrors Homebrew behaviour).
// The overall function only returns an error for hard failures (e.g. can't
// read the home directory).
func AutoUpdateIfStale(outW, errW io.Writer) error {
	if os.Getenv("KIMCHI_NO_AUTO_UPDATE") != "" {
		return nil
	}

	interval := defaultAutoUpdateSecs
	if v := os.Getenv("KIMCHI_AUTO_UPDATE_SECS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			interval = n
		}
	}

	stampPath, err := autoUpdateStampPath()
	if err != nil {
		return err
	}

	if !isStale(stampPath, interval) {
		return nil
	}

	// Ensure the default cookbook is cloned before we attempt to pull it.
	if err := EnsureDefault(outW); err != nil {
		fmt.Fprintf(errW, "warning: ensure default cookbook: %v\n", err)
		// Non-fatal — continue with whatever cookbooks are available.
	}

	cookbooks, err := Load()
	if err != nil || len(cookbooks) == 0 {
		// No cookbooks registered — nothing to do, but touch the stamp so we
		// don't attempt again on every invocation.
		_ = touchStamp(stampPath)
		return nil
	}

	fmt.Fprintln(outW, "==> Auto-updating cookbooks…")
	for _, cb := range cookbooks {
		if err := Pull(cb.Path); err != nil {
			fmt.Fprintf(errW, "warning: auto-update %s: %v\n", cb.Name, err)
		}
	}

	return touchStamp(stampPath)
}

// isStale returns true when the stamp file is older than intervalSecs seconds
// (or does not exist yet).
func isStale(stampPath string, intervalSecs int) bool {
	info, err := os.Stat(stampPath)
	if err != nil {
		return true // missing → treat as stale
	}
	return time.Since(info.ModTime()) > time.Duration(intervalSecs)*time.Second
}

// TouchAutoUpdateStamp resets the auto-update cooldown, so the next invocation
// won't pull cookbooks again. Call this after an explicit `kimchi update`.
func TouchAutoUpdateStamp() error {
	p, err := autoUpdateStampPath()
	if err != nil {
		return err
	}
	return touchStamp(p)
}

// touchStamp creates or updates the modification time of the stamp file.
func touchStamp(stampPath string) error {
	if err := os.MkdirAll(filepath.Dir(stampPath), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(stampPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	f.Close()
	now := time.Now()
	return os.Chtimes(stampPath, now, now)
}
