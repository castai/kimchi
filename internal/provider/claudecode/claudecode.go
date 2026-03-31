package claudecode

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/castai/kimchi/internal/config"
	"github.com/castai/kimchi/internal/gsd"
	"github.com/castai/kimchi/internal/tools"
)

const (
	logsExporter               = "otlp"
	logsProtocol               = "http/json"
	logsEndpoint               = "https://api.cast.ai/ai-optimizer/v1beta/logs:ingest"
	logsAuthorizationHeaderFmt = "Authorization=Bearer %s"
	logsExportInterval         = "5000"
)

// InjectGSD creates symlinks from ~/.claude/ to the kimchi-managed GSD
// directory. Returns the list of symlink paths that were created so they
// can be cleaned up after the process exits.
func InjectGSD() ([]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	kimchiGSD := filepath.Join(homeDir, ".config", "kimchi", "gsd", "claude-code")
	claudeDir := filepath.Join(homeDir, ".claude")

	var created []string
	if err := symlinkTree(kimchiGSD, claudeDir, &created); err != nil {
		return created, err
	}
	return created, nil
}

// CleanupGSD removes the symlinks created by InjectGSD.
func CleanupGSD(symlinks []string) {
	for _, path := range symlinks {
		// Only remove if it's still a symlink (don't delete real dirs).
		info, err := os.Lstat(path)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			os.Remove(path)
		}
	}
}

// symlinkTree symlinks directories from src into dst. If a target directory
// already exists as a real directory, it recurses into it to symlink children
// instead of skipping the whole tree. Created symlinks are appended to created.
func symlinkTree(src, dst string, created *[]string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read dir %s: %w", src, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		info, err := os.Lstat(dstPath)
		if err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			// Target exists as a real directory — recurse to symlink children.
			if err := symlinkTree(srcPath, dstPath, created); err != nil {
				return err
			}
			continue
		}

		if err := gsd.EnsureSymlink(srcPath, dstPath); err != nil {
			return err
		}
		*created = append(*created, dstPath)
	}

	return nil
}

// Env returns the environment variables needed to run Claude Code
// with Cast AI configuration. The apiKey is injected as ANTHROPIC_AUTH_TOKEN.
func Env(apiKey string, telemetryOptIn bool) map[string]string {
	env := map[string]string{
		"ANTHROPIC_BASE_URL":                     tools.AnthropicBaseURL,
		"ANTHROPIC_AUTH_TOKEN":                   apiKey,
		"ANTHROPIC_DEFAULT_OPUS_MODEL":           tools.ReasoningModel.Slug,
		"ANTHROPIC_DEFAULT_SONNET_MODEL":         tools.CodingModel.Slug,
		"ANTHROPIC_DEFAULT_HAIKU_MODEL":          tools.CodingModel.Slug,
		"CLAUDE_CODE_SUBAGENT_MODEL":             tools.CodingModel.Slug,
		"CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS": "1",
	}

	if telemetryOptIn {
		env["CLAUDE_CODE_ENABLE_TELEMETRY"] = "1"
		env["OTEL_LOGS_EXPORTER"] = logsExporter
		env["OTEL_EXPORTER_OTLP_LOGS_PROTOCOL"] = logsProtocol
		env["OTEL_EXPORTER_OTLP_LOGS_ENDPOINT"] = logsEndpoint
		env["OTEL_EXPORTER_OTLP_LOGS_HEADERS"] = fmt.Sprintf(logsAuthorizationHeaderFmt, apiKey)
		env["OTEL_LOGS_EXPORT_INTERVAL"] = logsExportInterval
	}

	return env
}

// WriteConfig writes the Claude Code settings file for the given scope,
// injecting Cast AI configuration.
func WriteConfig(scope config.ConfigScope, telemetryOptIn bool) error {
	apiKey, err := config.GetAPIKey()
	if err != nil {
		return fmt.Errorf("get API key: %w", err)
	}
	if apiKey == "" {
		return fmt.Errorf("API key not configured")
	}

	path, err := config.ScopePaths(scope, "~/.claude/settings.json")
	if err != nil {
		return fmt.Errorf("get config path: %w", err)
	}

	existing, err := config.ReadJSON(path)
	if err != nil {
		return fmt.Errorf("read existing settings: %w", err)
	}

	envSettings, _ := existing["env"].(map[string]any)
	if envSettings == nil {
		envSettings = make(map[string]any)
	}

	delete(envSettings, "ANTHROPIC_MODEL")

	for k, v := range Env(apiKey, telemetryOptIn) {
		envSettings[k] = v
	}

	existing["env"] = envSettings
	existing["model"] = "opusplan"

	existing["alwaysThinkingEnabled"] = true
	existing["autoCompactEnabled"] = true

	if err := config.WriteJSON(path, existing); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}

	return nil
}
