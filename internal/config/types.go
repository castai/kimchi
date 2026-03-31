package config

type Config struct {
	APIKey           string `json:"api_key"`
	TelemetryEnabled *bool  `json:"telemetry_enabled,omitempty"`
}
