package recipe

// Recipe is the top-level portable snapshot of an AI tool configuration.
// Version 1 supports OpenCode only.
type Recipe struct {
	Name        string   `yaml:"name"`
	Author      string   `yaml:"author"`
	Description string   `yaml:"description,omitempty"`
	Model       string   `yaml:"model"`
	UseCase     string   `yaml:"use_case"`
	Version     string   `yaml:"version"`
	Tools       ToolsMap `yaml:"tools"`
}

// ToolsMap holds per-tool configuration blocks. Fields are omitted when nil.
type ToolsMap struct {
	OpenCode *OpenCodeConfig `yaml:"opencode,omitempty"`
}

// OpenCodeConfig captures the exportable OpenCode settings (no secrets).
type OpenCodeConfig struct {
	Provider       OpenCodeProvider `yaml:"provider"`
	Model          string           `yaml:"model"`
	Compaction     CompactionConfig `yaml:"compaction"`
	AgentsMD       string           `yaml:"agents_md,omitempty"`
	Skills         []SkillEntry     `yaml:"skills,omitempty"`
	CustomCommands []CommandEntry   `yaml:"custom_commands,omitempty"`
	Agents         []AgentEntry     `yaml:"agents,omitempty"`
}

// OpenCodeProvider mirrors the provider block in opencode.json.
// apiKey is deliberately absent — secrets are never exported.
type OpenCodeProvider struct {
	Name    string                 `yaml:"name"`
	NPM     string                 `yaml:"npm"`
	Options OpenCodeProviderOptions `yaml:"options"`
	Models  map[string]ModelDef   `yaml:"models"`
}

// OpenCodeProviderOptions holds non-secret provider options.
// apiKey is deliberately absent.
type OpenCodeProviderOptions struct {
	BaseURL      string `yaml:"baseURL"`
	LitellmProxy bool   `yaml:"litellmProxy"`
}

// ModelDef describes a single model entry in the provider's models map.
type ModelDef struct {
	Name      string     `yaml:"name"`
	ToolCall  bool       `yaml:"tool_call"`
	Reasoning bool       `yaml:"reasoning,omitempty"`
	Limit     ModelLimit `yaml:"limit"`
}

// ModelLimit holds the token window constraints for a model.
type ModelLimit struct {
	Context int `yaml:"context"`
	Output  int `yaml:"output"`
}

// CompactionConfig holds context compaction settings.
type CompactionConfig struct {
	Auto bool `yaml:"auto"`
}

// SkillEntry is a named SKILL.md file from ~/.config/opencode/skills/<name>/.
type SkillEntry struct {
	Name    string `yaml:"name"`
	Content string `yaml:"content"`
}

// CommandEntry is a named *.md file from ~/.config/opencode/commands/.
type CommandEntry struct {
	Name    string `yaml:"name"`
	Content string `yaml:"content"`
}

// AgentEntry is a named *.md file from ~/.config/opencode/agents/.
type AgentEntry struct {
	Name    string `yaml:"name"`
	Content string `yaml:"content"`
}
