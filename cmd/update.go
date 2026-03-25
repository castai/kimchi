package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

			res, err := update.Check(ctx, client, version.Version)
			if err != nil {
				return fmt.Errorf("check for updates: %w", err)
			}

			if !res.LatestVersion.GreaterThan(&res.CurrentVersion) {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Already up to date (%s)\n", res.CurrentVersion.String())
				return nil
			}

			execPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolve executable path: %w", err)
			}
			if resolved, err := filepath.EvalSymlinks(execPath); err == nil {
				execPath = resolved
			} else {
				return fmt.Errorf("resolve symlinks for %s: %w", execPath, err)
			}

			permCheck := selfupdate.Options{TargetPath: execPath}
			if err := permCheck.CheckPermissions(); err != nil {
				return fmt.Errorf("cannot update %s: permission denied (try running with sudo)", execPath)
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

			versionTag := "v" + res.LatestVersion.String()
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Updating kimchi %s → %s...\n", res.CurrentVersion.String(), res.LatestVersion.String())
			if err := update.Apply(ctx, client, versionTag, update.WithExecutablePath(execPath), update.WithProgressWriter(os.Stderr)); err != nil {
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
