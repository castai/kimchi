package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/castai/kimchi/internal/config"
	"github.com/castai/kimchi/internal/provider/codex"
	"github.com/castai/kimchi/internal/tools"
)

func NewCodexCommand() *cobra.Command {
	return &cobra.Command{
		Use:                "codex [flags and args for codex]",
		Short:              "Launch Codex with Cast AI models",
		Long:               "Wraps the Codex CLI, injecting Cast AI configuration via environment variables.\nAll arguments are passed through to the codex binary.",
		DisableFlagParsing: true,
		SilenceUsage:       true,
		SilenceErrors:      true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			apiKey, err := config.ResolveAPIKey(cfg)
			if err != nil {
				return err
			}

			printBanner(os.Stderr, "codex", cfg)

			env, err := codex.Env(apiKey)
			if err != nil {
				return fmt.Errorf("prepare codex environment: %w", err)
			}

			return tools.ExecTool("codex", args, env)
		},
	}
}
