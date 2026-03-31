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
	return writeClaudeCode(scope, true)
}

func writeClaudeCode(scope config.ConfigScope, telemetryOptIn bool) error {
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

	envVars := map[string]string{
		"ANTHROPIC_BASE_URL":                     AnthropicBaseURL,
		"ANTHROPIC_AUTH_TOKEN":                   apiKey,
		"ANTHROPIC_DEFAULT_OPUS_MODEL":           ReasoningModel.Slug,
		"ANTHROPIC_DEFAULT_SONNET_MODEL":         CodingModel.Slug,
		"ANTHROPIC_DEFAULT_HAIKU_MODEL":          CodingModel.Slug,
		"CLAUDE_CODE_SUBAGENT_MODEL":             CodingModel.Slug,
		"CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS": "1",
	}

	if telemetryOptIn {
		envVars["CLAUDE_CODE_ENABLE_TELEMETRY"] = "1"
		envVars["OTEL_LOGS_EXPORTER"] = logsExporter
		envVars["OTEL_EXPORTER_OTLP_LOGS_PROTOCOL"] = logsProtocol
		envVars["OTEL_EXPORTER_OTLP_LOGS_ENDPOINT"] = logsEndpoint
		envVars["OTEL_EXPORTER_OTLP_LOGS_HEADERS"] = fmt.Sprintf(logsAuthorizationHeaderFmt, apiKey)
		envVars["OTEL_LOGS_EXPORT_INTERVAL"] = logsExportInterval
	}

	for k, v := range envVars {
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
