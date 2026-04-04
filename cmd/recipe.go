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
	cmd.AddCommand(NewRecipeInstallCommand())
	return cmd
}

func NewRecipeInstallCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "install [file]",
		Short: "Install an AI tool configuration from a recipe file",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := ""
			if len(args) > 0 {
				filePath = args[0]
			}
			return tui.RunInstallWizard(filePath)
		},
	}
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
