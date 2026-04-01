package telemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew_EmptyAPIKey_ReturnsNoopClient(t *testing.T) {
	client := New("", true, "test-device-id")
	assert.IsType(t, &noopClient{}, client, "expected noopClient when API key is empty")
}

func TestNew_TelemetryDisabled_ReturnsNoopClient(t *testing.T) {
	client := New("some-api-key", false, "test-device-id")
	assert.IsType(t, &noopClient{}, client, "expected noopClient when telemetry is disabled")
}
