package config

type Config struct {
	APIKey           string `json:"api_key,omitempty"`
	TelemetryEnabled *bool  `json:"telemetry_enabled,omitempty"`
	DeviceID         string `json:"device_id,omitempty"`
}
