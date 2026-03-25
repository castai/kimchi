package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/castai/kimchi/internal/update"
	"github.com/castai/kimchi/internal/version"
)

func NewUpdateCommand() *cobra.Command {
	var (
		force  bool
		dryRun bool
	)

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update kimchi to the latest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client := update.NewGitHubClient()

			res, err := update.Check(ctx, client, version.Version)
			if err != nil {
				return fmt.Errorf("check for updates: %w", err)
			}

			if !res.LatestVersion.GreaterThan(&res.CurrentVersion) {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Already up to date (%s)\n", res.CurrentVersion.String())
				return nil
			}

			execPath, err := update.ResolveExecutablePath()
			if err != nil {
				return err
			}

			if err := update.CheckPermissions(execPath); err != nil {
				return err
			}

			if dryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Update available: %s → %s\n", res.CurrentVersion.String(), res.LatestVersion.String())
				return nil
			}

			if !force {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Update available: %s → %s\nContinue? [Y/n]: ", res.CurrentVersion.String(), res.LatestVersion.String())
				reader := bufio.NewReader(os.Stdin)
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer != "" && answer != "y" && answer != "yes" {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Update cancelled.")
					return nil
				}
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Updating kimchi %s → %s...\n", res.CurrentVersion.String(), res.LatestVersion.String())
			if err := update.Apply(ctx, client, res.LatestTag, update.WithExecutablePath(execPath), update.WithProgressWriter(os.Stderr)); err != nil {
				return err
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "✓ Successfully updated to", res.LatestVersion.String())
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be updated without making changes")

	return cmd
}
