package telemetry

import (
	"testing"
)

func TestNew_EmptyAPIKey_ReturnsNoopClient(t *testing.T) {
	t.Setenv("KIMCHI_TELEMETRY", "true")

	client := New("")
	if _, ok := client.(*noopClient); !ok {
		t.Errorf("expected noopClient when API key is empty, got %T", client)
	}
}
