package cmd

import (
	"context"
	"flag"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"github.com/castai/kimchi/internal/telemetry"
	"github.com/castai/kimchi/internal/tui"
	"github.com/castai/kimchi/internal/version"
)

var (
	debug     bool
	verbose   bool
	telClient telemetry.Client
)

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "kimchi",
		Short: "Configure AI coding tools to use Cast AI open-source models",
		Long: `Kimchi by Cast AI
Connect your AI tools to powerful open-source models

This tool configures AI coding assistants (OpenCode, Claude Code, Cursor, etc.)
to use Cast AI's serverless inference endpoints with optimal model selection:

Model Selection Strategy:
  • kimi-k2.5     - Primary model: reasoning, planning, code generation, and images
                    262K context • 32K output • Vision + reasoning
  • glm-5-fp8     - Coding subagent: writing, refactoring, and debugging code
                    202K context • 32K output • Strategic thinking
  • minimax-m2.5  - Secondary subagent: available across all tool installations
                    196K context • 32K output • Optimized for coding

Each tool is automatically configured with the best model for its use case,
removing the complexity of manual model selection while ensuring peak performance.

Get your API key at: https://kimchi.console.cast.ai`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.String(),
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			propagetKlogFlags(cmd)

			telClient = telemetry.New()
			cmd.SetContext(telemetry.WithCtx(cmd.Context(), telClient))
			telClient.Track(telemetry.NewEvent("app_started", nil))
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			if telClient != nil {
				telClient.Close()
			}

			klog.Flush()

		},
		RunE: runConfigure,
	}

	root.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug output")
	root.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose output")

	initKlogFlags(root)

	root.AddCommand(NewVersionCommand())
	root.AddCommand(NewCompletionCommand())
	root.AddCommand(NewUpdateCommand())
	root.AddCommand(NewConfigCommand())

	return root
}

// initKlogFlags registers klog verbosity flags (-v) on the root command
func initKlogFlags(root *cobra.Command) {
	fs := flag.NewFlagSet("klog", flag.ContinueOnError)
	klog.InitFlags(fs)
	root.PersistentFlags().AddGoFlagSet(fs)
}

// propagetKlogFlags applies the --debug or --verbose flag values to klog's -v verbosity level
func propagetKlogFlags(cmd *cobra.Command) {
	vLevel := "0"
	if verbose {
		vLevel = "2"
	} else if debug {
		vLevel = "1"
	}

	if vLevel != "0" {
		err := cmd.Flags().Set("v", vLevel)
		if err != nil {
			klog.Warningf("Failed to set verbosity level: %v", err)
		}
	}
}

func runConfigure(cmd *cobra.Command, args []string) error {
	_, err := tui.RunWizard(cmd.Context())
	return err
}

// Execute runs the root command.
func Execute() error {
	root := newRootCommand()
	return root.ExecuteContext(context.Background())
}
