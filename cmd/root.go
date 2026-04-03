package cmd

import (
	"context"
	"flag"
	"fmt"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"github.com/castai/kimchi/internal/config"
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
		RunE:          runConfigure,
	}

	root.PersistentPreRun = func(cmd *cobra.Command, _ []string) {
		propagateKlogFlags(root)
		initTelemetry(root)
		cmd.SetContext(root.Context())
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

func initTelemetry(root *cobra.Command) {
	cfg, loadErr := config.Load()
	if loadErr != nil {
		klog.V(1).ErrorS(loadErr, "failed to load config")
		cfg = &config.Config{}
	}
	original := *cfg

	enabled, err := config.IsTelemetryEnabledFromConfig(cfg)
	if err != nil {
		klog.V(1).ErrorS(err, "failed to check telemetry status, assuming disabled")
		enabled = false
	}

	if cfg.DeviceID == "" {
		cfg.DeviceID = uuid.NewString()
	}

	client := telemetry.New(telemetry.PostHogAPIKey, enabled, cfg.DeviceID)
	root.SetContext(telemetry.WithCtx(root.Context(), client))

	if !cfg.TelemetryNoticeShown && enabled {
		fmt.Fprintln(root.ErrOrStderr(),
			"INFO: Kimchi collects anonymous usage data to improve the product. "+
				"Run 'kimchi config telemetry off' to disable.")
		cfg.TelemetryNoticeShown = true
	}

	if loadErr == nil && !original.Equal(cfg) {
		if saveErr := config.Save(cfg); saveErr != nil {
			klog.V(1).ErrorS(saveErr, "failed to save config")
		}
	}

	// Note: app_started fires on all commands, including "config telemetry off".
	// This is intentional — telemetry is still enabled at the time the event fires.
	// The client is a noop when telemetry is already disabled.
	client.Track(telemetry.NewEvent("app_started", nil))
}

// propagateKlogFlags applies the --debug or --verbose flag values to klog's -v verbosity level
func propagateKlogFlags(cmd *cobra.Command) {
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
func Execute(args ...string) error {
	root := newRootCommand()
	if len(args) > 0 {
		root.SetArgs(args)
	}
	ctx := context.Background()
	defer func() {
		telemetry.FromCtx(root.Context()).Close()
		klog.Flush()
	}()
	return root.ExecuteContext(ctx)
}
