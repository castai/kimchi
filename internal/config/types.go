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
	APIKey          string     `json:"api_key"`
	Mode            ConfigMode `json:"mode,omitempty"`
	SelectedTools   []string   `json:"selected_tools,omitempty"`
	Scope           string     `json:"scope,omitempty"`
	TelemetryOptIn  bool       `json:"telemetry_opt_in"`
	GSDInstalledFor []string   `json:"gsd_installed_for,omitempty"`
}
