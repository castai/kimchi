package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/castai/kimchi/internal/cookbook"
)

func NewCookbookCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cookbook",
		Short: "Manage recipe cookbooks",
	}
	cmd.AddCommand(NewCookbookAddCommand())
	cmd.AddCommand(NewCookbookCreateCommand())
	cmd.AddCommand(NewCookbookListCommand())
	cmd.AddCommand(NewCookbookUpdateCommand())
	return cmd
}

func NewCookbookAddCommand() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "add <url>",
		Short: "Clone and register a cookbook from a git URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := args[0]
			if name == "" {
				name = cookbook.NameFromURL(url)
			}

			cloneBase, err := cookbook.DefaultCloneDir()
			if err != nil {
				return err
			}
			destDir := filepath.Join(cloneBase, name)

			if cookbook.IsRepo(destDir) {
				fmt.Fprintf(cmd.OutOrStdout(), "Directory %s already exists, skipping clone.\n", destDir)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Cloning %s into %s…\n", url, destDir)
				if err := cookbook.Clone(url, destDir); err != nil {
					return fmt.Errorf("clone cookbook: %w", err)
				}
			}

			cb := cookbook.Cookbook{Name: name, URL: url, Path: destDir}
			if err := cookbook.Add(cb); err != nil {
				return fmt.Errorf("register cookbook: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "✓ Cookbook %q registered (%s)\n", name, destDir)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Override the cookbook name (defaults to repo name)")
	return cmd
}

func NewCookbookCreateCommand() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "create <url>",
		Short: "Scaffold a new cookbook and register it",
		Long: `Create scaffolds a minimal cookbook structure in a new local directory,
pushes it to the given remote URL, and registers it.

The remote repository must already exist (empty or otherwise).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := args[0]
			if name == "" {
				name = cookbook.NameFromURL(url)
			}

			cloneBase, err := cookbook.DefaultCloneDir()
			if err != nil {
				return err
			}
			destDir := filepath.Join(cloneBase, name)

			if cookbook.IsRepo(destDir) {
				fmt.Fprintf(cmd.OutOrStdout(), "Directory %s already exists, skipping clone.\n", destDir)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Cloning %s into %s…\n", url, destDir)
				if err := cookbook.Clone(url, destDir); err != nil {
					return fmt.Errorf("clone remote: %w", err)
				}
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Scaffolding cookbook structure…")
			if err := cookbook.Scaffold(destDir, name); err != nil {
				return fmt.Errorf("scaffold: %w", err)
			}

			cb := cookbook.Cookbook{Name: name, URL: url, Path: destDir}
			if err := cookbook.Add(cb); err != nil {
				return fmt.Errorf("register cookbook: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "✓ Cookbook %q created and registered (%s)\n", name, destDir)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Override the cookbook name (defaults to repo name)")
	return cmd
}

func NewCookbookListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List registered cookbooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			cookbooks, err := cookbook.Load()
			if err != nil {
				return err
			}
			if len(cookbooks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No cookbooks registered. Use `kimchi cookbook add <url>` to add one.")
				return nil
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%-20s  %-50s  %s\n", "NAME", "URL", "PATH")
			fmt.Fprintf(w, "%-20s  %-50s  %s\n", "----", "---", "----")
			for _, cb := range cookbooks {
				name := cb.Name
				if cookbook.IsDefault(cb.Name) {
					name += " (default)"
				}
				fmt.Fprintf(w, "%-20s  %-50s  %s\n", name, cb.URL, cb.Path)
			}
			return nil
		},
	}
}

func NewCookbookUpdateCommand() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "update [name]",
		Short: "Pull latest changes for registered cookbooks",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				name = args[0]
			}

			cookbooks, err := cookbook.Load()
			if err != nil {
				return err
			}

			updated := 0
			for _, cb := range cookbooks {
				if name != "" && cb.Name != name {
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Updating %s…\n", cb.Name)
				if err := cookbook.Pull(cb.Path); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "  warning: %v\n", err)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s updated\n", cb.Name)
				updated++
			}

			if updated == 0 && name != "" {
				return fmt.Errorf("cookbook %q not found", name)
			}
			return nil
		},
	}

	return cmd
}
