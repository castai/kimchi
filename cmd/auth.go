package cmd

import (
	"github.com/spf13/cobra"

	"github.com/castai/kimchi/internal/tui"
)

func NewAuthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Cast AI authentication",
	}
	cmd.AddCommand(NewAuthLoginCommand())
	return cmd
}

func NewAuthLoginCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate with Cast AI and save your API key",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.RunAuthWizard()
		},
	}
}
