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
	debug   bool
	verbose bool
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
			if debug {
				klog.SetLogger(klog.LoggerWithValues(klog.Background(), "debug", true))
			}
			if verbose {
				klog.SetLogger(klog.LoggerWithValues(klog.Background(), "verbose", true))
			}
		},
		RunE: runConfigure,
	}

	root.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug output")
	root.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose output")
	initKlog(root)

	root.AddCommand(NewVersionCommand())
	root.AddCommand(NewCompletionCommand())
	root.AddCommand(NewUpdateCommand())
	root.AddCommand(NewConfigCommand())

	return root
}

func initKlog(root *cobra.Command) {
	fs := flag.NewFlagSet("klog", flag.ContinueOnError)
	klog.InitFlags(fs)
	root.PersistentFlags().AddGoFlagSet(fs)
}

func runConfigure(cmd *cobra.Command, args []string) error {
	_, err := tui.RunWizard(cmd.Context())
	return err
}

// Execute runs the root command with the given telemetry client.
func Execute() error {
	telClient := telemetry.New()
	defer telClient.Close()

	telClient.Track(telemetry.NewEvent("app_started", nil))

	root := newRootCommand()
	ctx := telemetry.WithCtx(context.Background(), telClient)
	return root.ExecuteContext(ctx)
}
