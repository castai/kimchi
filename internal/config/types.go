package config

// ConfigMode determines how kimchi configures tools.
type ConfigMode string

const (
	// ModeInject configures tools at runtime via environment variables and managed config files.
	// No changes are made to the tool's own configuration directories.
	ModeInject ConfigMode = "inject"

	// ModeOverride writes configuration directly into each tool's config files
	// (e.g. ~/.config/opencode/opencode.json). Tools can then be run directly.
	ModeOverride ConfigMode = "override"
)

type Config struct {
	APIKey               string     `json:"api_key,omitempty"`
	Mode                 ConfigMode `json:"mode,omitempty"`
	SelectedTools        []string   `json:"selected_tools,omitempty"`
	Scope                string     `json:"scope,omitempty"`
	TelemetryEnabled     *bool      `json:"telemetry_enabled,omitempty"`
	DeviceID             string     `json:"device_id,omitempty"`
	TelemetryNoticeShown bool       `json:"telemetry_notice_shown,omitempty"`
	GSDInstalledFor      []string   `json:"gsd_installed_for,omitempty"`
}

func (c *Config) Clone() Config {
	clone := *c
	if c.TelemetryEnabled != nil {
		v := *c.TelemetryEnabled
		clone.TelemetryEnabled = &v
	}
	return clone
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
