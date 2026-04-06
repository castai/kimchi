package update

import (
	"context"
	"fmt"
	"io"
	"os"
)

// AutoSelfUpdateIfNeeded checks whether a newer kimchi release is available
// and, if so, attempts to apply it automatically.
//
// It is a no-op when:
//   - KIMCHI_NO_AUTO_UPDATE is set to any non-empty value
//   - The cached version check is still fresh (no extra API call is made)
//   - The current binary is already up to date
//
// When a new version is found but the process lacks write permission to the
// binary, a one-line notice is printed instead of failing.
// All errors are treated as non-fatal so the caller's command still runs.
func AutoSelfUpdateIfNeeded(ctx context.Context, currentVersion string, outW, errW io.Writer) {
	if os.Getenv("KIMCHI_NO_AUTO_UPDATE") != "" {
		return
	}

	client := NewGitHubClient()
	res, err := Check(ctx, client, currentVersion) // uses 24h cached state
	if err != nil {
		return // network unavailable or parse error — skip silently
	}

	if !res.LatestVersion.GreaterThan(&res.CurrentVersion) {
		return // already up to date
	}

	execPath, err := ResolveExecutablePath()
	if err != nil {
		return
	}

	if err := CheckPermissions(execPath); err != nil {
		// Can't write the binary — just notify the user.
		fmt.Fprintf(outW, "==> A new release of kimchi is available: %s → %s\n",
			res.CurrentVersion.String(), res.LatestVersion.String())
		fmt.Fprintln(outW, "    Reinstall or run with elevated permissions to update.")
		return
	}

	fmt.Fprintf(outW, "==> Updating kimchi %s → %s…\n",
		res.CurrentVersion.String(), res.LatestVersion.String())
	if err := Apply(ctx, client, res.LatestTag, WithExecutablePath(execPath)); err != nil {
		fmt.Fprintf(errW, "warning: auto-update failed: %v\n", err)
		return
	}
	fmt.Fprintf(outW, "==> Updated to kimchi %s\n", res.LatestVersion.String())
}
