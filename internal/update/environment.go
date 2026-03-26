package update

import "os"

// IsUpdateCheckDisabled checks if update notifications are disabled.
// Any non-empty value disables checks, matching gh CLI's GH_NO_UPDATE_NOTIFIER convention.
func IsUpdateCheckDisabled() bool {
	return os.Getenv("KIMCHI_NO_UPDATE_CHECK") != ""
}
