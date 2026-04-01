package config

type Config struct {
	APIKey               string `json:"api_key,omitempty"`
	TelemetryEnabled     *bool  `json:"telemetry_enabled,omitempty"`
	DeviceID             string `json:"device_id,omitempty"`
	TelemetryNoticeShown bool   `json:"telemetry_notice_shown,omitempty"`
}

func (c *Config) Equal(other *Config) bool {
	if c.APIKey != other.APIKey ||
		c.DeviceID != other.DeviceID ||
		c.TelemetryNoticeShown != other.TelemetryNoticeShown {
		return false
	}
	if c.TelemetryEnabled == nil && other.TelemetryEnabled == nil {
		return true
	}
	if c.TelemetryEnabled == nil || other.TelemetryEnabled == nil {
		return false
	}
	return *c.TelemetryEnabled == *other.TelemetryEnabled
}
