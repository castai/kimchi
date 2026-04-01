package claudecode

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/castai/kimchi/internal/tools"
)

func TestEnv_SetsAllRequiredVars(t *testing.T) {
	env := Env("test-key", false)

	assert.Equal(t, "test-key", env["ANTHROPIC_AUTH_TOKEN"])
	assert.Equal(t, tools.MainModel.Slug, env["ANTHROPIC_DEFAULT_OPUS_MODEL"])
	assert.Equal(t, tools.CodingModel.Slug, env["ANTHROPIC_DEFAULT_SONNET_MODEL"])
	assert.Equal(t, tools.CodingModel.Slug, env["ANTHROPIC_DEFAULT_HAIKU_MODEL"])
	assert.Equal(t, tools.CodingModel.Slug, env["CLAUDE_CODE_SUBAGENT_MODEL"])
	assert.Equal(t, "1", env["CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS"])
}

func TestEnv_TelemetryEnabled(t *testing.T) {
	env := Env("test-key", true)

	assert.Equal(t, "1", env["CLAUDE_CODE_ENABLE_TELEMETRY"])
	assert.NotEmpty(t, env["OTEL_LOGS_EXPORTER"])
	assert.NotEmpty(t, env["OTEL_EXPORTER_OTLP_LOGS_PROTOCOL"])
	assert.NotEmpty(t, env["OTEL_EXPORTER_OTLP_LOGS_ENDPOINT"])
	assert.Contains(t, env["OTEL_EXPORTER_OTLP_LOGS_HEADERS"], "test-key")
	assert.NotEmpty(t, env["OTEL_LOGS_EXPORT_INTERVAL"])
}

func TestEnv_TelemetryDisabled(t *testing.T) {
	env := Env("test-key", false)

	_, hasEnableTelemetry := env["CLAUDE_CODE_ENABLE_TELEMETRY"]
	assert.False(t, hasEnableTelemetry, "CLAUDE_CODE_ENABLE_TELEMETRY should not be set when telemetry is disabled")

	_, hasOtelExporter := env["OTEL_LOGS_EXPORTER"]
	assert.False(t, hasOtelExporter, "OTEL_LOGS_EXPORTER should not be set when telemetry is disabled")
}

func TestEnv_ApiKeyInjectedIntoAuthHeader(t *testing.T) {
	env := Env("my-secret-key", true)

	assert.Contains(t, env["OTEL_EXPORTER_OTLP_LOGS_HEADERS"], "my-secret-key")
	assert.Equal(t, "my-secret-key", env["ANTHROPIC_AUTH_TOKEN"])
}
