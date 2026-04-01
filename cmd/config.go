package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/castai/kimchi/internal/config"
)

func NewConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage kimchi configuration",
		Long:  `View and modify kimchi configuration settings.`,
	}

	cmd.AddCommand(NewConfigTelemetryCommand())

	return cmd
}

func NewConfigTelemetryCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "telemetry [on|off]",
		Short: "Manage telemetry settings",
		Long:  `Enable or disable anonymous usage telemetry for kimchi. Run without arguments to show current status.`,
		Args:  cobra.MaximumNArgs(1),
		Example: `  kimchi config telemetry on    # enable telemetry
  kimchi config telemetry off   # disable telemetry
  kimchi config telemetry       # show current status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Show status
			if len(args) == 0 {
				return showTelemetryStatus()
			}

			// Set value using flexible parser
			enabled, err := config.ParseSwitch(args[0])
			if err != nil {
				return err
			}
			return setTelemetry(enabled)
		},
	}
}

func setTelemetry(enabled bool) error {
	if err := config.SetTelemetryEnabled(enabled); err != nil {
		return fmt.Errorf("failed to %s telemetry: %w", formatStatus(enabled), err)
	}

	fmt.Printf("Telemetry %s\n", formatStatus(enabled))
	return nil
}

func showTelemetryStatus() error {
	enabled, err := config.IsTelemetryEnabled()
	if err != nil {
		return fmt.Errorf("failed to get telemetry status: %w", err)
	}

	// Check if env var is set for display purposes
	envVal := os.Getenv(config.EnvTelemetry)
	if envVal != "" {
		fmt.Printf("Telemetry: %s (from %s=%s, overrides config)\n",
			formatStatus(enabled), config.EnvTelemetry, envVal)
		return nil
	}

	fmt.Printf("Telemetry: %s (from config)\n", formatStatus(enabled))
	return nil
}

func formatStatus(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}
