package cmd

import (
	"fmt"
	"os"
	"slices"

	"github.com/spf13/cobra"

	"github.com/castai/kimchi/internal/config"
	"github.com/castai/kimchi/internal/provider/claudecode"
	"github.com/castai/kimchi/internal/tools"
)

func NewClaudeCommand() *cobra.Command {
	return &cobra.Command{
		Use:                "claude [flags and args for claude]",
		Short:              "Launch Claude Code with Cast AI models",
		Long:               "Wraps the Claude Code CLI, injecting Cast AI configuration via environment variables.\nAll arguments are passed through to the claude binary.",
		DisableFlagParsing: true,
		SilenceUsage:       true,
		SilenceErrors:      true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			apiKey := cfg.APIKey
			if envKey := os.Getenv("KIMCHI_API_KEY"); envKey != "" {
				apiKey = envKey
			}
			if apiKey == "" {
				return fmt.Errorf("no API key configured — run 'kimchi' to set up, or set KIMCHI_API_KEY")
			}

			printBanner(os.Stderr, "claude", cfg)

			env := claudecode.Env(apiKey, cfg.TelemetryOptIn)

			var cleanup func()
			if slices.Contains(cfg.GSDInstalledFor, "claude-code") {
				created, err := claudecode.InjectGSD()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not inject GSD agents: %v\n", err)
				}
				if len(created) > 0 {
					cleanup = func() {
						claudecode.CleanupGSD(created)
					}
				}
			}

			if cleanup != nil {
				return tools.RunTool("claude", args, env, cleanup)
			}
			return tools.ExecTool("claude", args, env)
		},
	}
}
