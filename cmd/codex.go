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
		Short:              "Launch Codex with Kimchi models",
		Long:               "Wraps the Codex CLI, injecting Kimchi configuration via environment variables.\nAll arguments are passed through to the codex binary.",
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

			fetchedModels, err := tools.FetchModels(cmd.Context(), apiKey)
			if err != nil {
				return fmt.Errorf("fetch models: %w", err)
			}

			modelCfg := tools.BuildModelConfig(fetchedModels, cfg.ModelMain, cfg.ModelCoding, cfg.ModelSub)

			printBanner(os.Stderr, "codex", cfg, modelCfg)

			env, err := codex.Env(apiKey, modelCfg)
			if err != nil {
				return fmt.Errorf("prepare codex environment: %w", err)
			}

			return tools.ExecTool("codex", args, env)
		},
	}
}
