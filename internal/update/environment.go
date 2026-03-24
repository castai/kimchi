package update

import "os"

var ciEnvVars = []string{
	"CI",
	"GITHUB_ACTIONS",
	"BUILD_NUMBER",
	"RUN_ID",
}

func IsCI() bool {
	for _, env := range ciEnvVars {
		if os.Getenv(env) != "" {
			return true
		}
	}
	return false
}

// IsUpdateCheckDisabled checks if update notifications are disabled.
// Any non-empty value disables checks, matching gh CLI's GH_NO_UPDATE_NOTIFIER convention.
func IsUpdateCheckDisabled() bool {
	return os.Getenv("KIMCHI_NO_UPDATE_CHECK") != ""
}
