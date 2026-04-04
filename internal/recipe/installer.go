package recipe

import (
	"fmt"
	"os"
	"path/filepath"

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

// Conflict describes a markdown asset that already exists on disk.
type Conflict struct {
	Kind string // "agents_md" | "skill" | "command" | "agent"
	Name string // empty for "agents_md"; asset name otherwise
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

	if oc.AgentsMD != "" {
		p := filepath.Join(base, "AGENTS.md")
		if _, err := os.Stat(p); err == nil {
			conflicts = append(conflicts, Conflict{Kind: "agents_md", Path: p})
		}
	}

	for _, s := range oc.Skills {
		p := filepath.Join(base, "skills", s.Name, "SKILL.md")
		if _, err := os.Stat(p); err == nil {
			conflicts = append(conflicts, Conflict{Kind: "skill", Name: s.Name, Path: p})
		}
	}

	for _, c := range oc.CustomCommands {
		p := filepath.Join(base, "commands", c.Name+".md")
		if _, err := os.Stat(p); err == nil {
			conflicts = append(conflicts, Conflict{Kind: "command", Name: c.Name, Path: p})
		}
	}

	for _, a := range oc.Agents {
		p := filepath.Join(base, "agents", a.Name+".md")
		if _, err := os.Stat(p); err == nil {
			conflicts = append(conflicts, Conflict{Kind: "agent", Name: a.Name, Path: p})
		}
	}

	return conflicts, nil
}

// AssetDecisions maps each Conflict.Path → true (overwrite) or false (skip).
// Non-conflicting assets (not in this map) are always written.
type AssetDecisions map[string]bool

// InstallOpenCode writes the recipe's OpenCode config to opencode.json and markdown
// assets to the appropriate paths. apiKey is injected into the provider options
// (it is never stored in a recipe). decisions controls overwrite behaviour for
// files that already exist on disk.
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

	// Build provider models map from recipe ModelDef structs.
	providerModels := make(map[string]any, len(oc.Provider.Models))
	for slug, m := range oc.Provider.Models {
		entry := map[string]any{
			"name":      m.Name,
			"tool_call": m.ToolCall,
			"limit": map[string]any{
				"context": m.Limit.Context,
				"output":  m.Limit.Output,
			},
		}
		if m.Reasoning {
			entry["reasoning"] = true
		}
		providerModels[slug] = entry
	}

	providerBlock := map[string]any{
		"npm":  oc.Provider.NPM,
		"name": oc.Provider.Name,
		"options": map[string]any{
			"baseURL":      oc.Provider.Options.BaseURL,
			"litellmProxy": oc.Provider.Options.LitellmProxy,
			"apiKey":       apiKey,
		},
		"models": providerModels,
	}

	providers, _ := existing["provider"].(map[string]any)
	if providers == nil {
		providers = make(map[string]any)
	}
	providers[tools.ProviderName] = providerBlock
	existing["provider"] = providers
	existing["model"] = oc.Model
	existing["compaction"] = map[string]any{"auto": oc.Compaction.Auto}
	existing["$schema"] = "https://opencode.ai/config.json"

	if err := config.WriteJSON(jsonPath, existing); err != nil {
		return fmt.Errorf("write opencode config: %w", err)
	}

	// ── Markdown assets ───────────────────────────────────────────────────────

	if oc.AgentsMD != "" {
		p := filepath.Join(base, "AGENTS.md")
		if shouldWrite(p, decisions) {
			if err := config.WriteFile(p, []byte(oc.AgentsMD)); err != nil {
				return fmt.Errorf("write AGENTS.md: %w", err)
			}
		}
	}

	for _, s := range oc.Skills {
		p := filepath.Join(base, "skills", s.Name, "SKILL.md")
		if shouldWrite(p, decisions) {
			if err := config.WriteFile(p, []byte(s.Content)); err != nil {
				return fmt.Errorf("write skill %s: %w", s.Name, err)
			}
		}
	}

	for _, c := range oc.CustomCommands {
		p := filepath.Join(base, "commands", c.Name+".md")
		if shouldWrite(p, decisions) {
			if err := config.WriteFile(p, []byte(c.Content)); err != nil {
				return fmt.Errorf("write command %s: %w", c.Name, err)
			}
		}
	}

	for _, a := range oc.Agents {
		p := filepath.Join(base, "agents", a.Name+".md")
		if shouldWrite(p, decisions) {
			if err := config.WriteFile(p, []byte(a.Content)); err != nil {
				return fmt.Errorf("write agent %s: %w", a.Name, err)
			}
		}
	}

	return nil
}

// shouldWrite returns true if the path should be written.
// Paths not in decisions (i.e. no pre-existing conflict) are always written.
// Paths in decisions are written only if the value is true (overwrite).
func shouldWrite(path string, decisions AssetDecisions) bool {
	if v, inMap := decisions[path]; inMap {
		return v
	}
	return true
}
