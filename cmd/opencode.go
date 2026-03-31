package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/castai/kimchi/internal/config"
	"github.com/castai/kimchi/internal/provider/opencode"
	"github.com/castai/kimchi/internal/tools"
)

func NewOpenCodeCommand() *cobra.Command {
	return &cobra.Command{
		Use:                "opencode [flags and args for opencode]",
		Short:              "Launch OpenCode with Cast AI models",
		Long:               "Wraps the OpenCode CLI, injecting Cast AI configuration via environment variables.\nAll arguments are passed through to the opencode binary.",
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

			printBanner(os.Stderr, "opencode", cfg)

			env, err := opencode.Env(apiKey)
			if err != nil {
				return fmt.Errorf("prepare opencode environment: %w", err)
			}

			return tools.ExecTool("opencode", args, env)
		},
	}
}
