package recipe

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/castai/kimchi/internal/config"
	"github.com/castai/kimchi/internal/tools"
)

// ReadFromFile parses a recipe YAML file and returns the Recipe.
func ReadFromFile(path string) (*Recipe, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	var r Recipe
	if err := yaml.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse recipe: %w", err)
	}
	if r.Name == "" {
		return nil, fmt.Errorf("invalid recipe: missing name")
	}
	if r.Tools.OpenCode == nil {
		return nil, fmt.Errorf("invalid recipe: no supported tool configuration found")
	}
	return &r, nil
}

// Conflict describes a file that already exists on disk and would be overwritten.
type Conflict struct {
	Kind string // "agents_md" | "skill" | "command" | "agent" | "theme" | "plugin" | "tool" | "ref" | "tui"
	Name string // human-readable label; empty for single-file kinds like "agents_md"
	Path string // absolute path shown to the user
}

// DetectConflicts returns which of the recipe's embedded assets already exist on disk.
// Only assets present in the recipe are checked — others are never reported.
func DetectConflicts(r *Recipe) ([]Conflict, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	base := filepath.Join(homeDir, ".config", "opencode")

	oc := r.Tools.OpenCode
	if oc == nil {
		return nil, nil
	}

	var conflicts []Conflict
	check := func(kind, name, path string) {
		if _, err := os.Stat(path); err == nil {
			conflicts = append(conflicts, Conflict{Kind: kind, Name: name, Path: path})
		}
	}

	if oc.AgentsMD != "" {
		check("agents_md", "", filepath.Join(base, "AGENTS.md"))
	}

	for _, s := range oc.Skills {
		skillDir := filepath.Join(base, "skills", s.Name)
		check("skill", s.Name, filepath.Join(skillDir, "SKILL.md"))
		for _, f := range s.Files {
			check("skill", s.Name+"/"+f.Path, filepath.Join(skillDir, filepath.FromSlash(f.Path)))
		}
	}

	for _, c := range oc.CustomCommands {
		check("command", c.Name, filepath.Join(base, "commands", c.Name+".md"))
	}

	for _, a := range oc.Agents {
		check("agent", a.Name, filepath.Join(base, "agents", a.Name+".md"))
	}

	if oc.TUI != nil {
		check("tui", "", filepath.Join(base, "tui.json"))
	}

	for _, f := range oc.ThemeFiles {
		check("theme", f.Path, filepath.Join(base, "themes", f.Path))
	}

	for _, f := range oc.PluginFiles {
		check("plugin", f.Path, filepath.Join(base, "plugins", filepath.FromSlash(f.Path)))
	}

	for _, f := range oc.ToolFiles {
		check("tool", f.Path, filepath.Join(base, "tools", filepath.FromSlash(f.Path)))
	}

	for _, f := range oc.ReferencedFiles {
		check("ref", f.Path, filepath.Join(base, filepath.FromSlash(f.Path)))
	}

	return conflicts, nil
}

// AssetDecisions maps each Conflict.Path → true (overwrite) or false (skip).
// Non-conflicting assets (not in this map) are always written.
type AssetDecisions map[string]bool

// InstallOpenCode writes the recipe's OpenCode config to opencode.json and all
// embedded assets to the appropriate paths. apiKey is injected into the kimchi
// provider's options block (it is never stored in a recipe). decisions controls
// overwrite behaviour for files that already exist on disk.
func InstallOpenCode(r *Recipe, apiKey string, decisions AssetDecisions) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	base := filepath.Join(homeDir, ".config", "opencode")
	oc := r.Tools.OpenCode

	// ── opencode.json ────────────────────────────────────────────────────────

	jsonPath := filepath.Join(base, "opencode.json")
	existing, err := config.ReadJSON(jsonPath)
	if err != nil {
		return fmt.Errorf("read existing opencode config: %w", err)
	}

	// Providers — use the map from the recipe verbatim, but inject the kimchi API key.
	if oc.Providers != nil {
		providers := deepCopyMap(oc.Providers)
		injectAPIKey(providers, tools.ProviderName, apiKey)
		existing["provider"] = providers
	}

	setIfNonZero(existing, "model", oc.Model)
	setIfNonZero(existing, "small_model", oc.SmallModel)
	setIfNonZero(existing, "default_agent", oc.DefaultAgent)
	setIfNonNil(existing, "compaction", oc.Compaction)
	setIfNonNil(existing, "agent", oc.AgentConfigs)
	setIfNonNil(existing, "mcp", oc.MCP)
	setIfNonNil(existing, "tools", oc.Tools)
	setIfNonNil(existing, "experimental", oc.Experimental)
	setIfNonNil(existing, "command", oc.InlineCommands)
	if oc.Permission != nil {
		existing["permission"] = oc.Permission
	}
	if oc.Formatter != nil {
		existing["formatter"] = oc.Formatter
	}
	if oc.LSP != nil {
		existing["lsp"] = oc.LSP
	}
	if oc.Snapshot != nil {
		existing["snapshot"] = *oc.Snapshot
	}
	if len(oc.DisabledProviders) > 0 {
		existing["disabled_providers"] = oc.DisabledProviders
	}
	if len(oc.EnabledProviders) > 0 {
		existing["enabled_providers"] = oc.EnabledProviders
	}
	if len(oc.Plugin) > 0 {
		existing["plugin"] = oc.Plugin
	}
	if len(oc.Instructions) > 0 {
		existing["instructions"] = oc.Instructions
	}
	existing["$schema"] = "https://opencode.ai/config.json"

	if err := config.WriteJSON(jsonPath, existing); err != nil {
		return fmt.Errorf("write opencode config: %w", err)
	}

	// ── tui.json ─────────────────────────────────────────────────────────────

	if oc.TUI != nil {
		tuiPath := filepath.Join(base, "tui.json")
		if shouldWrite(tuiPath, decisions) {
			if err := config.WriteJSON(tuiPath, tuiConfigToMap(oc.TUI)); err != nil {
				return fmt.Errorf("write tui.json: %w", err)
			}
		}
	}

	// ── AGENTS.md ─────────────────────────────────────────────────────────────

	if oc.AgentsMD != "" {
		p := filepath.Join(base, "AGENTS.md")
		if shouldWrite(p, decisions) {
			if err := config.WriteFile(p, []byte(oc.AgentsMD)); err != nil {
				return fmt.Errorf("write AGENTS.md: %w", err)
			}
		}
	}

	// ── Skills ───────────────────────────────────────────────────────────────

	for _, s := range oc.Skills {
		skillDir := filepath.Join(base, "skills", s.Name)
		p := filepath.Join(skillDir, "SKILL.md")
		if shouldWrite(p, decisions) {
			if err := config.WriteFile(p, []byte(s.Content)); err != nil {
				return fmt.Errorf("write skill %s: %w", s.Name, err)
			}
		}
		for _, f := range s.Files {
			fp := filepath.Join(skillDir, filepath.FromSlash(f.Path))
			if shouldWrite(fp, decisions) {
				if err := config.WriteFile(fp, []byte(f.Content)); err != nil {
					return fmt.Errorf("write skill file %s/%s: %w", s.Name, f.Path, err)
				}
			}
		}
	}

	// ── Custom commands ───────────────────────────────────────────────────────

	for _, c := range oc.CustomCommands {
		p := filepath.Join(base, "commands", c.Name+".md")
		if shouldWrite(p, decisions) {
			if err := config.WriteFile(p, []byte(c.Content)); err != nil {
				return fmt.Errorf("write command %s: %w", c.Name, err)
			}
		}
	}

	// ── Agents ───────────────────────────────────────────────────────────────

	for _, a := range oc.Agents {
		p := filepath.Join(base, "agents", a.Name+".md")
		if shouldWrite(p, decisions) {
			if err := config.WriteFile(p, []byte(a.Content)); err != nil {
				return fmt.Errorf("write agent %s: %w", a.Name, err)
			}
		}
	}

	// ── Theme / plugin / tool files ───────────────────────────────────────────

	for _, f := range oc.ThemeFiles {
		p := filepath.Join(base, "themes", f.Path)
		if shouldWrite(p, decisions) {
			if err := config.WriteFile(p, []byte(f.Content)); err != nil {
				return fmt.Errorf("write theme %s: %w", f.Path, err)
			}
		}
	}

	for _, f := range oc.PluginFiles {
		p := filepath.Join(base, "plugins", filepath.FromSlash(f.Path))
		if shouldWrite(p, decisions) {
			if err := config.WriteFile(p, []byte(f.Content)); err != nil {
				return fmt.Errorf("write plugin file %s: %w", f.Path, err)
			}
		}
	}

	for _, f := range oc.ToolFiles {
		p := filepath.Join(base, "tools", filepath.FromSlash(f.Path))
		if shouldWrite(p, decisions) {
			if err := config.WriteFile(p, []byte(f.Content)); err != nil {
				return fmt.Errorf("write tool file %s: %w", f.Path, err)
			}
		}
	}

	// ── @-referenced files ────────────────────────────────────────────────────

	for _, f := range oc.ReferencedFiles {
		p := filepath.Join(base, filepath.FromSlash(f.Path))
		if shouldWrite(p, decisions) {
			if err := config.WriteFile(p, []byte(f.Content)); err != nil {
				return fmt.Errorf("write referenced file %s: %w", f.Path, err)
			}
		}
	}

	return nil
}

// injectAPIKey replaces the kimchi:secret: placeholder in the named provider's
// options.apiKey with the real API key.
func injectAPIKey(providers map[string]any, providerName, apiKey string) {
	prov, ok := providers[providerName].(map[string]any)
	if !ok {
		return
	}
	opts, ok := prov["options"].(map[string]any)
	if !ok {
		return
	}
	for _, key := range []string{"apiKey", "api_key", "token", "secret"} {
		if v, ok := opts[key].(string); ok && strings.HasPrefix(v, SecretPlaceholderPrefix) {
			opts[key] = apiKey
		}
	}
}

// deepCopyMap returns a shallow copy of m so the recipe struct is not mutated.
func deepCopyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// setIfNonZero writes key=value to m only when value is not the zero string.
func setIfNonZero(m map[string]any, key, value string) {
	if value != "" {
		m[key] = value
	}
}

// setIfNonNil writes key=value to m only when value is non-nil.
func setIfNonNil(m map[string]any, key string, value map[string]any) {
	if value != nil {
		m[key] = value
	}
}

// tuiConfigToMap converts a TUIConfig struct back to a map suitable for WriteJSON.
func tuiConfigToMap(t *TUIConfig) map[string]any {
	m := make(map[string]any)
	if t.Theme != "" {
		m["theme"] = t.Theme
	}
	if t.ScrollSpeed != 0 {
		m["scroll_speed"] = t.ScrollSpeed
	}
	if t.ScrollAcceleration != nil {
		m["scroll_acceleration"] = t.ScrollAcceleration
	}
	if t.DiffStyle != "" {
		m["diff_style"] = t.DiffStyle
	}
	if len(t.Keybinds) > 0 {
		m["keybinds"] = t.Keybinds
	}
	return m
}

// shouldWrite returns true if the path should be written.
// Paths not in decisions (no pre-existing conflict) are always written.
// Paths in decisions are written only if the value is true (overwrite).
func shouldWrite(path string, decisions AssetDecisions) bool {
	if v, inMap := decisions[path]; inMap {
		return v
	}
	return true
}
