package telemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew_EmptyAPIKey_ReturnsNoopClient(t *testing.T) {
	t.Setenv("KIMCHI_TELEMETRY", "true")

	client := New("")
	assert.IsType(t, &noopClient{}, client, "expected noopClient when API key is empty")
}

func TestNew_InvalidTelemetryEnvVar_ReturnsNoopClient(t *testing.T) {
	t.Setenv("KIMCHI_TELEMETRY", "banana")

	client := New("some-api-key")
	assert.IsType(t, &noopClient{}, client, "expected noopClient (fail closed) on invalid KIMCHI_TELEMETRY value")
}
