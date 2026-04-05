package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/castai/kimchi/internal/cookbook"
	"github.com/castai/kimchi/internal/recipe"
)

// NewUpgradeCommand returns the top-level `kimchi upgrade` command.
// It is the equivalent of `brew update && brew upgrade`:
//  1. Pull the latest changes for all registered cookbooks.
//  2. Upgrade all non-pinned installed recipes to their latest versions.
func NewUpgradeCommand() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Update cookbooks and upgrade all installed recipes",
		Long: `Upgrade is a one-shot equivalent of:

  kimchi cookbook update   # pull latest recipes from all cookbooks
  kimchi recipe upgrade --yes  # reinstall any recipes that have newer versions

Recipes that require interactive secret entry are skipped with a warning.
Pinned recipes are never upgraded.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			// ── Step 1: update all cookbooks ────────────────────────────────────
			cookbooks, err := cookbook.Load()
			if err != nil {
				return fmt.Errorf("load cookbooks: %w", err)
			}

			if len(cookbooks) == 0 {
				fmt.Fprintln(w, "No cookbooks registered. Use `kimchi cookbook add <url>` to add one.")
			} else {
				fmt.Fprintln(w, "Updating cookbooks…")
				for _, cb := range cookbooks {
					fmt.Fprintf(w, "  %s… ", cb.Name)
					if dryRun {
						fmt.Fprintln(w, "(dry-run, skipped)")
						continue
					}
					if err := cookbook.Pull(cb.Path); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", err)
					} else {
						fmt.Fprintln(w, "✓")
					}
				}
			}

			// ── Step 2: find upgradeable recipes ────────────────────────────────
			installed, err := recipe.LoadInstalled()
			if err != nil {
				return fmt.Errorf("load installed recipes: %w", err)
			}
			if len(installed) == 0 {
				fmt.Fprintln(w, "No installed recipes found.")
				return nil
			}

			type upgradeCandidate struct {
				installed recipe.InstalledRecipe
				latest    recipe.RecipeRef
			}

			var candidates []upgradeCandidate
			for _, inst := range installed {
				if inst.Pinned {
					continue
				}
				source := inst.Name
				if inst.Cookbook != "" {
					source = inst.Cookbook + "/" + inst.Name
				}
				ref, err := recipe.FindRecipe(source)
				if err != nil {
					continue
				}
				if recipe.CompareVersions(ref.Version, inst.Version) > 0 {
					candidates = append(candidates, upgradeCandidate{installed: inst, latest: *ref})
				}
			}

			if len(candidates) == 0 {
				fmt.Fprintln(w, "All recipes are up to date.")
				return nil
			}

			fmt.Fprintln(w, "\nAvailable recipe upgrades:")
			for _, c := range candidates {
				fmt.Fprintf(w, "  %-30s  %s → %s\n", c.installed.Name, c.installed.Version, c.latest.Version)
			}

			if dryRun {
				return nil
			}

			// ── Step 3: upgrade each recipe non-interactively ───────────────────
			fmt.Fprintln(w, "\nUpgrading recipes…")
			for _, c := range candidates {
				fmt.Fprintf(w, "  %s@%s… ", c.latest.Name, c.latest.Version)
				err := recipe.InstallHeadless(recipe.HeadlessInstallOptions{
					Source:             c.latest.Path,
					OverwriteConflicts: true,
				})
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "skipped: %v\n", err)
				} else {
					fmt.Fprintln(w, "✓")
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be updated without making changes")
	return cmd
}
