package tools

import (
	"fmt"

	"github.com/castai/kimchi/internal/config"
)

const (
	logsExporter               = "otlp"
	logsProtocol               = "http/json"
	logsEndpoint               = "https://api.cast.ai/ai-optimizer/v1beta/logs:ingest"
	logsAuthorizationHeaderFmt = "Authorization=Bearer %s"
	logsExportInterval         = "5000"
)

func init() {
	register(Tool{
		ID:          ToolClaudeCode,
		Name:        "Claude Code",
		Description: "Anthropic's coding agent",
		ConfigPath:  "~/.claude/settings.json",
		BinaryName:  "claude",
		IsInstalled: detectBinary("claude"),
		Write:       writeClaudeCodeDefault,
	})
}

func writeClaudeCodeDefault(scope config.ConfigScope) error {
	return WriteClaudeCode(scope, true)
}

func WriteClaudeCode(scope config.ConfigScope, telemetryOptIn bool) error {
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

	envSettings["ANTHROPIC_BASE_URL"] = anthropicBaseURL
	envSettings["ANTHROPIC_AUTH_TOKEN"] = apiKey
	envSettings["ANTHROPIC_DEFAULT_OPUS_MODEL"] = MainModel.Slug
	envSettings["ANTHROPIC_DEFAULT_SONNET_MODEL"] = CodingModel.Slug
	envSettings["ANTHROPIC_DEFAULT_HAIKU_MODEL"] = CodingModel.Slug
	envSettings["CLAUDE_CODE_SUBAGENT_MODEL"] = CodingModel.Slug
	envSettings["CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS"] = "1"

	if telemetryOptIn {
		envSettings["CLAUDE_CODE_ENABLE_TELEMETRY"] = "1"
		envSettings["OTEL_LOGS_EXPORTER"] = logsExporter
		envSettings["OTEL_EXPORTER_OTLP_LOGS_PROTOCOL"] = logsProtocol
		envSettings["OTEL_EXPORTER_OTLP_LOGS_ENDPOINT"] = logsEndpoint
		envSettings["OTEL_EXPORTER_OTLP_LOGS_HEADERS"] = fmt.Sprintf(logsAuthorizationHeaderFmt, apiKey)
		envSettings["OTEL_LOGS_EXPORT_INTERVAL"] = logsExportInterval
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
