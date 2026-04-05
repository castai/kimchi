package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/castai/kimchi/internal/recipe"
	"github.com/castai/kimchi/internal/tui"
)

func NewRecipeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recipe",
		Short: "Manage kimchi recipes",
	}
	cmd.AddCommand(NewRecipeExportCommand())
	cmd.AddCommand(NewRecipeInstallCommand())
	cmd.AddCommand(NewRecipePushCommand())
	cmd.AddCommand(NewRecipeForkCommand())
	cmd.AddCommand(NewRecipeListCommand())
	cmd.AddCommand(NewRecipeSearchCommand())
	cmd.AddCommand(NewRecipeInfoCommand())
	cmd.AddCommand(NewRecipeUpgradeCommand())
	cmd.AddCommand(NewRecipePinCommand())
	cmd.AddCommand(NewRecipeUnpinCommand())
	return cmd
}

// ── install ──────────────────────────────────────────────────────────────────

func NewRecipeInstallCommand() *cobra.Command {
	var noApply bool

	cmd := &cobra.Command{
		Use:   "install [source]",
		Short: "Install a recipe",
		Long: `Install a recipe from a local file or a registered cookbook.

Examples:
  kimchi recipe install ./my-recipe.yaml          local file
  kimchi recipe install python-debugger           by name
  kimchi recipe install python-debugger@1.2.0     specific version
  kimchi recipe install alice/python-debugger     cookbook/name`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			source := ""
			if len(args) > 0 {
				source = args[0]
			}
			return tui.RunInstallWizard(tui.InstallWizardOptions{
				Source:  source,
				NoApply: noApply,
			})
		},
	}

	cmd.Flags().BoolVar(&noApply, "no-apply", false, "Preview the recipe without writing any files")
	return cmd
}

// ── export ───────────────────────────────────────────────────────────────────

func NewRecipeExportCommand() *cobra.Command {
	var outputPath string
	var name string
	var tags []string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export your current AI tool configuration as a recipe",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.RunExportWizard(tui.ExportWizardOptions{
				OutputPath: outputPath,
				Name:       name,
				Tags:       tags,
				DryRun:     dryRun,
			})
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path (default: prompted in wizard)")
	cmd.Flags().StringVar(&name, "name", "", "Recipe name (skips the name prompt)")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Tag to add to the recipe (repeatable)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the recipe to stdout without writing a file")
	return cmd
}

// ── push ─────────────────────────────────────────────────────────────────────

func NewRecipePushCommand() *cobra.Command {
	var (
		cookbookName string
		patch        bool
		minor        bool
		major        bool
		meta         bool
		dryRun       bool
	)

	cmd := &cobra.Command{
		Use:   "push <file>",
		Short: "Publish a recipe to its cookbook",
		Long: `Push commits a recipe to its target cookbook and creates a version tag.

If you do not have write access to the cookbook's remote, kimchi will
authenticate with GitHub via device flow, fork the repo, and open a pull request.

Version bump flags:
  --patch   1.2.3 → 1.2.4  (backwards-compatible bug fixes)
  --minor   1.2.3 → 1.3.0  (new functionality)
  --major   1.2.3 → 2.0.0  (breaking changes)
  --meta    metadata-only change, no version bump required`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			count := 0
			for _, b := range []bool{patch, minor, major} {
				if b {
					count++
				}
			}
			if count > 1 {
				return fmt.Errorf("specify at most one of --patch, --minor, --major")
			}

			return recipe.Push(recipe.PushOptions{
				File:         args[0],
				CookbookName: cookbookName,
				Patch:        patch,
				Minor:        minor,
				Major:        major,
				Meta:         meta,
				DryRun:       dryRun,
			}, func(msg string) {
				fmt.Fprintln(cmd.OutOrStdout(), msg)
			})
		},
	}

	cmd.Flags().StringVar(&cookbookName, "cookbook", "", "Cookbook to push to (default: from recipe yaml or auto-selected)")
	cmd.Flags().BoolVar(&patch, "patch", false, "Bump patch version")
	cmd.Flags().BoolVar(&minor, "minor", false, "Bump minor version")
	cmd.Flags().BoolVar(&major, "major", false, "Bump major version")
	cmd.Flags().BoolVar(&meta, "meta", false, "Metadata-only push (no version bump required)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be pushed without making changes")
	return cmd
}

// ── fork ─────────────────────────────────────────────────────────────────────

func NewRecipeForkCommand() *cobra.Command {
	var (
		newName    string
		outputPath string
	)

	cmd := &cobra.Command{
		Use:   "fork <source>",
		Short: "Fork a recipe to create your own customisable copy",
		Long: `Fork copies a recipe and marks it as forked from the original.
The forked copy starts at version 0.1.0 and has no cookbook set
(the cookbook will be resolved on first push).

Source may be a file path, recipe name, cookbook/name, or name@version.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			outPath, err := recipe.Fork(recipe.ForkOptions{
				Source:     args[0],
				NewName:    newName,
				OutputPath: outputPath,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Forked recipe written to %s\n", outPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&newName, "name", "", "Override the recipe name in the fork")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path (default: <name>.yaml)")
	return cmd
}

// ── list ─────────────────────────────────────────────────────────────────────

func NewRecipeListCommand() *cobra.Command {
	var cookbookFilter string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available recipes from registered cookbooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			refs, err := recipe.ListAll()
			if err != nil {
				return err
			}
			if len(refs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No recipes found. Use `kimchi cookbook add <url>` to register a cookbook.")
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tVERSION\tCOOKBOOK\tAUTHOR")
			fmt.Fprintln(w, "----\t-------\t--------\t------")
			for _, r := range refs {
				if cookbookFilter != "" && r.Cookbook != cookbookFilter {
					continue
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Name, r.Version, r.Cookbook, r.Author)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&cookbookFilter, "cookbook", "", "Filter by cookbook name")
	return cmd
}

// ── search ───────────────────────────────────────────────────────────────────

func NewRecipeSearchCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search recipes by name, description, or tag",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			refs, err := recipe.Search(args[0])
			if err != nil {
				return err
			}
			if len(refs) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No recipes found matching %q\n", args[0])
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tVERSION\tCOOKBOOK\tAUTHOR")
			fmt.Fprintln(w, "----\t-------\t--------\t------")
			for _, r := range refs {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Name, r.Version, r.Cookbook, r.Author)
			}
			return w.Flush()
		},
	}
}

// ── info ─────────────────────────────────────────────────────────────────────

func NewRecipeInfoCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "info <source>",
		Short: "Show details about a recipe",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := recipe.ResolveSource(args[0])
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Name:        %s\n", r.Name)
			fmt.Fprintf(w, "Version:     %s\n", r.Version)
			if r.Author != "" {
				fmt.Fprintf(w, "Author:      %s\n", r.Author)
			}
			if r.Cookbook != "" {
				fmt.Fprintf(w, "Cookbook:    %s\n", r.Cookbook)
			}
			if r.Description != "" {
				fmt.Fprintf(w, "Description: %s\n", r.Description)
			}
			if len(r.Tags) > 0 {
				fmt.Fprintf(w, "Tags:        %s\n", strings.Join(r.Tags, ", "))
			}
			if r.UseCase != "" {
				fmt.Fprintf(w, "Use case:    %s\n", r.UseCase)
			}
			if r.ForkedFrom != nil {
				fmt.Fprintf(w, "Forked from: %s/%s@%s\n", r.ForkedFrom.Author, r.ForkedFrom.Cookbook, r.ForkedFrom.Version)
			}
			if r.CreatedAt != "" {
				fmt.Fprintf(w, "Created:     %s\n", r.CreatedAt)
			}
			if r.UpdatedAt != "" {
				fmt.Fprintf(w, "Updated:     %s\n", r.UpdatedAt)
			}
			return nil
		},
	}
}

// ── upgrade ──────────────────────────────────────────────────────────────────

func NewRecipeUpgradeCommand() *cobra.Command {
	var (
		cookbookFilter string
		dryRun         bool
		yes            bool
	)

	cmd := &cobra.Command{
		Use:   "upgrade [name]",
		Short: "Upgrade installed recipes to their latest cookbook versions",
		Long: `Upgrade checks all installed (non-pinned) recipes for newer versions
in their cookbooks and offers to reinstall them.

When a recipe name is provided only that recipe is checked.

With --yes, recipes are upgraded non-interactively: existing files are
overwritten and recipes requiring external secrets are skipped with a warning.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			installed, err := recipe.LoadInstalled()
			if err != nil {
				return err
			}
			if len(installed) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No installed recipes found.")
				return nil
			}

			type upgrade struct {
				installed recipe.InstalledRecipe
				latest    recipe.RecipeRef
			}

			var upgrades []upgrade
			for _, inst := range installed {
				if len(args) > 0 && inst.Name != args[0] {
					continue
				}
				if cookbookFilter != "" && inst.Cookbook != cookbookFilter {
					continue
				}
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
					upgrades = append(upgrades, upgrade{installed: inst, latest: *ref})
				}
			}

			if len(upgrades) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "All recipes are up to date.")
				return nil
			}

			w := cmd.OutOrStdout()
			fmt.Fprintln(w, "Available upgrades:")
			for _, u := range upgrades {
				fmt.Fprintf(w, "  %-30s  %s → %s\n", u.installed.Name, u.installed.Version, u.latest.Version)
			}

			if dryRun {
				return nil
			}

			if yes {
				for _, u := range upgrades {
					fmt.Fprintf(w, "\nUpgrading %s@%s…\n", u.latest.Name, u.latest.Version)
					err := recipe.InstallHeadless(recipe.HeadlessInstallOptions{
						Source:             u.latest.Path,
						OverwriteConflicts: true,
					})
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "  skipped: %v\n", err)
					} else {
						fmt.Fprintf(w, "  ✓ %s upgraded to %s\n", u.latest.Name, u.latest.Version)
					}
				}
				return nil
			}

			for _, u := range upgrades {
				fmt.Fprintf(w, "\nInstalling %s@%s…\n", u.latest.Name, u.latest.Version)
				if err := tui.RunInstallWizard(tui.InstallWizardOptions{
					Source: u.latest.Path,
				}); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "  error: %v\n", err)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&cookbookFilter, "cookbook", "", "Limit upgrades to a specific cookbook")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show available upgrades without installing")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Upgrade all non-interactively, overwriting existing files")
	return cmd
}

// ── pin / unpin ───────────────────────────────────────────────────────────────

func NewRecipePinCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "pin <name>",
		Short: "Pin a recipe to its current version (prevents upgrade)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := recipe.Pin(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ %s pinned\n", args[0])
			return nil
		},
	}
}

func NewRecipeUnpinCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "unpin <name>",
		Short: "Unpin a recipe so it can be upgraded",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := recipe.Unpin(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ %s unpinned\n", args[0])
			return nil
		},
	}
}
