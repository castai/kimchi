package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"github.com/castai/kimchi/internal/tui"
	"github.com/castai/kimchi/internal/version"
)

var (
	debug   bool
	verbose bool
)

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "kimchi",
		Short: "Configure AI coding tools to use Cast AI open-source models",
		Long: `Kimchi by Cast AI
Connect your AI tools to powerful open-source models

This tool configures AI coding assistants (OpenCode, Claude Code, Cursor, etc.)
to use Cast AI's serverless inference endpoints with models like:
  - glm-5-fp8 (reasoning/planning)
  - minimax-m2.5 (code generation)

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

	return root
}

func initKlog(root *cobra.Command) {
	fs := flag.NewFlagSet("klog", flag.ContinueOnError)
	klog.InitFlags(fs)
	root.PersistentFlags().AddGoFlagSet(fs)
}

func runConfigure(cmd *cobra.Command, args []string) error {
	_, err := tui.RunWizard()
	return err
}

func Execute() {
	root := NewRootCommand()
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
