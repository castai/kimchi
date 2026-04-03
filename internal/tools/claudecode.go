package tools

import "fmt"

const (
	logsExporter               = "otlp"
	logsProtocol               = "http/json"
	logsEndpoint               = "https://api.cast.ai/ai-optimizer/v1beta/logs:ingest"
	logsAuthorizationHeaderFmt = "Authorization=Bearer %s"
	logsExportInterval         = "5000"
)

// ClaudeCodeEnvVars is used by inject mode (cmd/claude.go) to configure
// Claude Code at runtime without modifying on-disk settings.

// ClaudeCodeEnvVars returns the environment variables needed to run Claude Code
// with Cast AI configuration (used by inject mode).
func ClaudeCodeEnvVars(apiKey string, telemetryOptIn bool) map[string]string {
	env := map[string]string{
		"ANTHROPIC_BASE_URL":                     anthropicBaseURL,
		"ANTHROPIC_AUTH_TOKEN":                   apiKey,
		"ANTHROPIC_DEFAULT_OPUS_MODEL":           MainModel.Slug,
		"ANTHROPIC_DEFAULT_SONNET_MODEL":         CodingModel.Slug,
		"ANTHROPIC_DEFAULT_HAIKU_MODEL":          CodingModel.Slug,
		"CLAUDE_CODE_SUBAGENT_MODEL":             CodingModel.Slug,
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
