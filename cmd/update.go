package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/castai/kimchi/internal/cookbook"
	"github.com/castai/kimchi/internal/update"
	"github.com/castai/kimchi/internal/version"
)

// NewUpdateCommand returns `kimchi update`, which mirrors `brew update`:
//  1. Pull the latest commits for all registered cookbooks.
//  2. Check for a new kimchi release and apply it if one is available.
//
// Both steps also run automatically in the background on every invocation
// (respecting KIMCHI_NO_AUTO_UPDATE and a 24h cooldown).
func NewUpdateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update cookbooks and kimchi itself",
		Long: `Update pulls the latest recipe commits from all registered cookbooks
and upgrades the kimchi binary if a newer release is available.

Both operations also run automatically in the background once per day.
Set KIMCHI_NO_AUTO_UPDATE=1 to disable all automatic updates.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			// ── 1. Pull all cookbooks ────────────────────────────────────────────
			cookbooks, err := cookbook.Load()
			if err != nil {
				return fmt.Errorf("load cookbooks: %w", err)
			}

			if len(cookbooks) == 0 {
				fmt.Fprintln(w, "No cookbooks registered. Use `kimchi cookbook add <url>` to add one.")
			} else {
				fmt.Fprintln(w, "==> Updating cookbooks…")
				for _, cb := range cookbooks {
					fmt.Fprintf(w, "    %s… ", cb.Name)
					if err := cookbook.Pull(cb.Path); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", err)
					} else {
						fmt.Fprintln(w, "✓")
					}
				}
				_ = cookbook.TouchAutoUpdateStamp() // reset the auto-update cooldown
			}

			// ── 2. Self-update the binary ────────────────────────────────────────
			fmt.Fprintln(w, "==> Checking for kimchi updates…")
			update.AutoSelfUpdateIfNeeded(cmd.Context(), version.Version, w, cmd.ErrOrStderr())

			return nil
		},
	}
}
