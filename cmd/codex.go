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

			apiKey := cfg.APIKey
			if envKey := os.Getenv("KIMCHI_API_KEY"); envKey != "" {
				apiKey = envKey
			}
			if apiKey == "" {
				return fmt.Errorf("no API key configured — run 'kimchi' to set up, or set KIMCHI_API_KEY")
			}

			scope := config.ConfigScope(cfg.Scope)
			if scope == "" {
				scope = config.ScopeGlobal
			}

			if err := ensureCodexConfig(scope); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not write codex config: %v\n", err)
			}

			printBanner(os.Stderr, "codex", cfg)

			env := codex.Env(apiKey)

			return tools.ExecTool("codex", args, env)
		},
	}
}

// ensureCodexConfig writes the kimchi provider and model catalog into
// ~/.codex/config.toml if the kimchi provider is not already defined there.
// This is a one-time setup: subsequent runs skip the write.
func ensureCodexConfig(scope config.ConfigScope) error {
	tool, ok := tools.ByID(tools.ToolCodex)
	if !ok || tool.Write == nil {
		return nil
	}

	configPath, err := config.ScopePaths(scope, "~/.codex/config.toml")
	if err != nil {
		return fmt.Errorf("get config path: %w", err)
	}

	existing, err := config.ReadTOML(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	providers, _ := existing["model_providers"].(map[string]any)
	if _, hasKimchi := providers[tools.ProviderName]; hasKimchi {
		return nil
	}

	return tool.Write(scope)
}
