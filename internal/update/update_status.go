package update

import (
	"github.com/Masterminds/semver/v3"
)

// UpdateStatus holds version-check data for any managed binary.
type UpdateStatus struct {
	DisplayName    string          // display name, e.g. "kimchi", "coding harness"
	CurrentVersion *semver.Version // can be nil if not installed
	LatestVersion  *semver.Version
	HasUpdate      bool
}

// Installed reports whether the binary is currently installed.
func (s UpdateStatus) Installed() bool {
	return s.CurrentVersion != nil
}
