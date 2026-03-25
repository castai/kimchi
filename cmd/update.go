package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/castai/kimchi/internal/update"
	"github.com/castai/kimchi/internal/version"
)

type updateDoneMsg struct{ err error }

type updateModel struct {
	spinner spinner.Model
	version string
	applyFn tea.Cmd
	err     error
	done    bool
}

func (m updateModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.applyFn)
}

func (m updateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case updateDoneMsg:
		m.done = true
		m.err = msg.err
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m updateModel) View() string {
	if m.done {
		return ""
	}
	return m.spinner.View() + " Updating to v" + m.version + "...\n"
}

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

			// Fail fast before downloading if we can't write to the executable.
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

			s := spinner.New()
			s.Spinner = spinner.Dot
			s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
			m := updateModel{
				spinner: s,
				version: res.LatestVersion.String(),
				applyFn: func() tea.Msg {
					return updateDoneMsg{err: update.Apply(ctx, client, res.LatestTag, update.WithExecutablePath(execPath))}
				},
			}
			finalModel, err := tea.NewProgram(m).Run()
			if err != nil {
				return fmt.Errorf("run update: %w", err)
			}
			if fm, ok := finalModel.(updateModel); ok && fm.err != nil {
				return fm.err
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "✓ Successfully updated to", res.LatestVersion.String())
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be updated without making changes")

	return cmd
}
