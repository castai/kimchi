package recipe

// Recipe is the top-level portable snapshot of an AI tool configuration.
type Recipe struct {
	// Header — identity and routing metadata
	Name      string `yaml:"name"`
	Version   string `yaml:"version"` // semver, e.g. "0.1.0"
	Cookbook  string `yaml:"cookbook,omitempty"`
	Author    string `yaml:"author,omitempty"`
	Description string      `yaml:"description,omitempty"`
	Tags        []string    `yaml:"tags,omitempty"`
	CreatedAt   string      `yaml:"created_at,omitempty"`
	UpdatedAt   string      `yaml:"updated_at,omitempty"`
	ForkedFrom  *ForkedFrom `yaml:"forked_from,omitempty"`

	// Convenience summary fields
	Model   string `yaml:"model,omitempty"`
	UseCase string `yaml:"use_case,omitempty"`

	// Tool-specific config blocks
	Tools ToolsMap `yaml:"tools"`
}

// ForkedFrom records the origin of a recipe that was created via `recipe fork`.
type ForkedFrom struct {
	Author   string `yaml:"author"`
	Cookbook string `yaml:"cookbook"`
	Version  string `yaml:"version"`
}

// ToolsMap holds per-tool configuration blocks. Fields are omitted when nil.
type ToolsMap struct {
	OpenCode *OpenCodeConfig `yaml:"opencode,omitempty"`
}

// SupportedToolNames returns the names of tools that have config blocks in this recipe.
func (t ToolsMap) SupportedToolNames() []string {
	var names []string
	if t.OpenCode != nil {
		names = append(names, "opencode")
	}
	return names
}

// OpenCodeConfig captures the exportable OpenCode settings.
// Secrets in providers and MCP servers are replaced with placeholder strings.
type OpenCodeConfig struct {
	// Provider / model (from opencode.json)
	Providers         map[string]any `yaml:"providers,omitempty"`
	Model             string         `yaml:"model,omitempty"`
	SmallModel        string         `yaml:"small_model,omitempty"`
	DefaultAgent      string         `yaml:"default_agent,omitempty"`
	DisabledProviders []string       `yaml:"disabled_providers,omitempty"`
	EnabledProviders  []string       `yaml:"enabled_providers,omitempty"`
	Plugin            []string       `yaml:"plugin,omitempty"`
	Snapshot          *bool          `yaml:"snapshot,omitempty"`

	// Behavior (from opencode.json)
	Compaction     map[string]any `yaml:"compaction,omitempty"`
	AgentConfigs   map[string]any `yaml:"agent,omitempty"`
	MCP            map[string]any `yaml:"mcp,omitempty"`
	Permission     any            `yaml:"permission,omitempty"`
	Tools          map[string]any `yaml:"tools,omitempty"`
	Experimental   map[string]any `yaml:"experimental,omitempty"`
	Formatter      any            `yaml:"formatter,omitempty"`
	LSP            any            `yaml:"lsp,omitempty"`
	InlineCommands map[string]any `yaml:"command,omitempty"`

	// TUI config (from tui.json) — optional, user-selectable
	TUI *TUIConfig `yaml:"tui,omitempty"`

	// Portable URL entries from the opencode.json instructions field.
	// Local path/glob entries are omitted — they are machine-specific.
	Instructions []string `yaml:"instructions,omitempty"`

	// Files discovered by resolving @path references inside exported markdown
	// content against ~/.config/opencode/. Stored so the installer can
	// recreate them in the right place.
	ReferencedFiles []FileEntry `yaml:"referenced_files,omitempty"`

	// File-based assets embedded into the recipe
	AgentsMD       string         `yaml:"agents_md,omitempty"`
	Skills         []SkillEntry   `yaml:"skills,omitempty"`
	CustomCommands []CommandEntry `yaml:"custom_commands,omitempty"`
	Agents         []AgentEntry   `yaml:"agents,omitempty"`
	ThemeFiles     []FileEntry    `yaml:"theme_files,omitempty"`
	PluginFiles    []FileEntry    `yaml:"plugin_files,omitempty"`
	ToolFiles      []FileEntry    `yaml:"tool_files,omitempty"`
}

// TUIConfig captures the exportable OpenCode TUI settings (tui.json).
type TUIConfig struct {
	Theme              string            `yaml:"theme,omitempty"`
	ScrollSpeed        float64           `yaml:"scroll_speed,omitempty"`
	ScrollAcceleration map[string]any    `yaml:"scroll_acceleration,omitempty"`
	DiffStyle          string            `yaml:"diff_style,omitempty"`
	Keybinds           map[string]string `yaml:"keybinds,omitempty"`
}

// SkillEntry is a skill directory from ~/.config/opencode/skills/<name>/.
// Content holds SKILL.md; Files holds any additional assets (scripts, etc.).
type SkillEntry struct {
	Name    string      `yaml:"name"`
	Content string      `yaml:"content"`
	Files   []FileEntry `yaml:"files,omitempty"`
}

// FileEntry is a file embedded from a config subdirectory.
// Path is relative to the containing directory (e.g. skill dir, themes/, plugins/).
type FileEntry struct {
	Path    string `yaml:"path"`
	Content string `yaml:"content"`
}

// CommandEntry is a named *.md file from ~/.config/opencode/commands/.
// Name may include a subdirectory prefix, e.g. "gsd/gsd-add-backlog".
type CommandEntry struct {
	Name    string `yaml:"name"`
	Content string `yaml:"content"`
}

// AgentEntry is a named *.md file from ~/.config/opencode/agents/.
type AgentEntry struct {
	Name    string `yaml:"name"`
	Content string `yaml:"content"`
}
