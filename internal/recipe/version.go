package recipe

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
)

// BumpPatch increments the patch component of a semver string (e.g. "1.2.3" → "1.2.4").
func BumpPatch(v string) (string, error) {
	sv, err := semver.NewVersion(v)
	if err != nil {
		return "", fmt.Errorf("parse version %q: %w", v, err)
	}
	next := sv.IncPatch()
	return next.Original(), nil
}

// BumpMinor increments the minor component (e.g. "1.2.3" → "1.3.0").
func BumpMinor(v string) (string, error) {
	sv, err := semver.NewVersion(v)
	if err != nil {
		return "", fmt.Errorf("parse version %q: %w", v, err)
	}
	next := sv.IncMinor()
	return next.Original(), nil
}

// BumpMajor increments the major component (e.g. "1.2.3" → "2.0.0").
func BumpMajor(v string) (string, error) {
	sv, err := semver.NewVersion(v)
	if err != nil {
		return "", fmt.Errorf("parse version %q: %w", v, err)
	}
	next := sv.IncMajor()
	return next.Original(), nil
}

// ValidateVersion returns an error if v is not a valid semver string.
func ValidateVersion(v string) error {
	_, err := semver.NewVersion(v)
	if err != nil {
		return fmt.Errorf("%q is not valid semver: %w", v, err)
	}
	return nil
}

// CompareVersions returns -1, 0, or 1 if a < b, a == b, or a > b.
// Invalid versions sort as less than valid ones.
func CompareVersions(a, b string) int {
	va, err1 := semver.NewVersion(a)
	vb, err2 := semver.NewVersion(b)
	if err1 != nil && err2 != nil {
		return 0
	}
	if err1 != nil {
		return -1
	}
	if err2 != nil {
		return 1
	}
	return va.Compare(vb)
}
