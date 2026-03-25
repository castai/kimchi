package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/minio/selfupdate"
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

			current, err := semver.NewVersion(version.Version)
			if err != nil {
				return fmt.Errorf("parse current version: %w", err)
			}

			info, err := client.LatestRelease(ctx)
			if err != nil {
				return fmt.Errorf("check for updates: %w", err)
			}

			latest, err := semver.NewVersion(info.TagName)
			if err != nil {
				return fmt.Errorf("parse latest version: %w", err)
			}

			if !latest.GreaterThan(current) {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Already up to date (%s)\n", current)
				return nil
			}

			execPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolve executable path: %w", err)
			}
			// Best-effort symlink resolution; the original path is usable if this fails.
			execPath, _ = filepath.EvalSymlinks(execPath)

			if dryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Update available: %s → %s\n", current, latest)
				return nil
			}

			permCheck := selfupdate.Options{TargetPath: execPath}
			if err := permCheck.CheckPermissions(); err != nil {
				return fmt.Errorf("cannot update %s: permission denied (try running with sudo)", execPath)
			}

			if !force {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Update available: %s → %s\nContinue? [Y/n]: ", current, latest)
				reader := bufio.NewReader(os.Stdin)
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer != "" && answer != "y" && answer != "yes" {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Update cancelled.")
					return nil
				}
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Updating kimchi %s → %s...\n", current, latest)
			if err := update.Apply(ctx, client, info.TagName, update.WithExecutablePath(execPath), update.WithProgressWriter(os.Stderr)); err != nil {
				return err
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "✓ Successfully updated to", latest)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be updated without making changes")

	return cmd
}
