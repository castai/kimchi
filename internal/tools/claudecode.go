package tools

import (
	"fmt"

	"github.com/castai/kimchi/internal/config"
)

const (
	LogsExporter               = "otlp"
	LogsProtocol               = "http/json"
	LogsEndpoint               = "https://api.cast.ai/ai-optimizer/v1beta/logs:ingest"
	LogsAuthorizationHeaderFmt = "Authorization=Bearer %s"
	LogsExportInterval         = "5000"
)

func init() {
	register(Tool{
		ID:          ToolClaudeCode,
		Name:        "Claude Code",
		Description: "Anthropic's coding agent",
		ConfigPath:  "~/.claude/settings.json",
		BinaryName:  "claude",
		IsInstalled: detectBinary("claude"),
		Write: func(scope config.ConfigScope) error {
			return WriteClaudeCodeConfig(scope, true)
		},
	})
}

// ClaudeCodeEnvVars returns the environment variables needed to run Claude Code
// with Cast AI configuration.
func ClaudeCodeEnvVars(apiKey string, telemetryOptIn bool) map[string]string {
	env := map[string]string{
		"ANTHROPIC_BASE_URL":                     AnthropicBaseURL,
		"ANTHROPIC_AUTH_TOKEN":                   apiKey,
		"ANTHROPIC_DEFAULT_OPUS_MODEL":           ReasoningModel.Slug,
		"ANTHROPIC_DEFAULT_SONNET_MODEL":         CodingModel.Slug,
		"ANTHROPIC_DEFAULT_HAIKU_MODEL":          CodingModel.Slug,
		"CLAUDE_CODE_SUBAGENT_MODEL":             CodingModel.Slug,
		"CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS": "1",
	}

	if telemetryOptIn {
		env["CLAUDE_CODE_ENABLE_TELEMETRY"] = "1"
		env["OTEL_LOGS_EXPORTER"] = LogsExporter
		env["OTEL_EXPORTER_OTLP_LOGS_PROTOCOL"] = LogsProtocol
		env["OTEL_EXPORTER_OTLP_LOGS_ENDPOINT"] = LogsEndpoint
		env["OTEL_EXPORTER_OTLP_LOGS_HEADERS"] = fmt.Sprintf(LogsAuthorizationHeaderFmt, apiKey)
		env["OTEL_LOGS_EXPORT_INTERVAL"] = LogsExportInterval
	}

	return env
}

// WriteClaudeCodeConfig writes the Claude Code settings file for the given scope.
func WriteClaudeCodeConfig(scope config.ConfigScope, telemetryOptIn bool) error {
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

	for k, v := range ClaudeCodeEnvVars(apiKey, telemetryOptIn) {
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
