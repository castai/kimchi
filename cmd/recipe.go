package cmd

import (
	"github.com/spf13/cobra"

	"github.com/castai/kimchi/internal/tui"
)

func NewRecipeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recipe",
		Short: "Manage kimchi recipes",
	}
	cmd.AddCommand(NewRecipeExportCommand())
	return cmd
}

func NewRecipeExportCommand() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export your current AI tool configuration as a portable recipe file",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.RunExportWizard(outputPath)
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path (default: prompted in wizard)")

	return cmd
}
