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

// InjectGSD creates symlinks from ~/.claude/{subdir} to the kimchi-managed GSD
// directory. Claude Code has no config-dir override, so symlinks into ~/.claude/
// are the only viable injection path. These symlinks persist after exit but are
// lightweight pointers to kimchi-managed content. If a target already exists
// (user's own install), it is left untouched.
func InjectGSD() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	kimchiGSD := filepath.Join(homeDir, ".config", "kimchi", "gsd", "claude-code")
	claudeDir := filepath.Join(homeDir, ".claude")

	entries, err := os.ReadDir(kimchiGSD)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read kimchi GSD dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		src := filepath.Join(kimchiGSD, entry.Name())
		target := filepath.Join(claudeDir, entry.Name())
		if err := gsd.EnsureSymlink(src, target); err != nil {
			return err
		}
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
