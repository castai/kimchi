package recipe

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/castai/kimchi/internal/config"
)

// OpenCodeAssets holds all data read from the local OpenCode installation.
// Missing files are silently skipped — nothing here is required.
type OpenCodeAssets struct {
	Config          map[string]any
	TUI             *TUIConfig
	AgentsMD        string
	Skills          []SkillEntry
	CustomCommands  []CommandEntry
	Agents          []AgentEntry
	ThemeFiles      []FileEntry
	PluginFiles     []FileEntry
	ToolFiles       []FileEntry
	// ReferencedFiles holds files found by resolving @path references inside
	// exported markdown content against the opencode config directory.
	ReferencedFiles []FileEntry
	// UnresolvedRefs lists @path references found in markdown content that
	// could not be resolved within the opencode config directory — typically
	// project-level paths the LLM will read at runtime.
	UnresolvedRefs []string
}

// ReadGlobalOpenCodeAssets reads all exportable assets from the user's global
// OpenCode configuration directory (~/.config/opencode/).
func ReadGlobalOpenCodeAssets() (*OpenCodeAssets, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	base := filepath.Join(homeDir, ".config", "opencode")
	return readAssets(
		filepath.Join(base, "opencode.json"),
		filepath.Join(base, "AGENTS.md"),
		base, // asset base: skills/, commands/, agents/, themes/, plugins/, tools/
		base, // ref resolution base (same for global)
	)
}

// ReadProjectOpenCodeAssets reads exportable assets from the current working
// directory's OpenCode project config.
//
// Project layout:
//   - ./opencode.json          — project config
//   - ./AGENTS.md              — project rules
//   - ./.opencode/skills/      — project skills
//   - ./.opencode/commands/    — project commands
//   - ./.opencode/agents/      — project agents
func ReadProjectOpenCodeAssets() (*OpenCodeAssets, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Config file: ./opencode.json (project root, not inside .opencode/)
	configFile := filepath.Join(cwd, "opencode.json")
	if _, err := os.Stat(configFile); errors.Is(err, fs.ErrNotExist) {
		// Fallback: .opencode/opencode.json
		configFile = filepath.Join(cwd, ".opencode", "opencode.json")
	}

	dotOpenCode := filepath.Join(cwd, ".opencode")

	return readAssets(
		configFile,
		filepath.Join(cwd, "AGENTS.md"),
		dotOpenCode, // asset base: .opencode/skills/, .opencode/commands/, etc.
		cwd,         // ref resolution base (resolve @-refs from project root)
	)
}

// ReadOpenCodeAssets is the legacy entry point kept for compatibility.
// New callers should use ReadGlobalOpenCodeAssets or ReadProjectOpenCodeAssets.
func ReadOpenCodeAssets() (*OpenCodeAssets, error) {
	return ReadGlobalOpenCodeAssets()
}

// readAssets is the shared implementation. Parameters:
//   - configFilePath: path to opencode.json
//   - agentsMDPath: path to AGENTS.md
//   - assetBase: root directory for skills/, commands/, agents/, themes/, plugins/, tools/
//   - refBase: root directory used when resolving @path references in markdown
func readAssets(configFilePath, agentsMDPath, assetBase, refBase string) (*OpenCodeAssets, error) {
	assets := &OpenCodeAssets{}

	// opencode.json
	cfg, err := config.ReadJSON(configFilePath)
	if err != nil {
		return nil, err
	}
	ScrubSecrets(cfg)
	assets.Config = cfg

	// tui.json — only meaningful for global scope; project scope returns nil here.
	tuiPath := filepath.Join(assetBase, "tui.json")
	if tuiRaw, err := config.ReadJSON(tuiPath); err == nil {
		assets.TUI = parseTUIConfig(tuiRaw)
	}

	// AGENTS.md
	if data, err := os.ReadFile(agentsMDPath); err == nil {
		assets.AgentsMD = string(data)
	}

	// skills/<name>/ — SKILL.md plus any extra files
	skillsDir := filepath.Join(assetBase, "skills")
	if entries, err := os.ReadDir(skillsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			skillDir := filepath.Join(skillsDir, e.Name())
			data, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
			if err != nil {
				continue
			}
			assets.Skills = append(assets.Skills, SkillEntry{
				Name:    e.Name(),
				Content: string(data),
				Files:   readExtraSkillFiles(skillDir),
			})
		}
	}

	// commands/ — supports nested subdirectories
	assets.CustomCommands = readMarkdownFilesRecursive(filepath.Join(assetBase, "commands"), "")

	// agents/*.md
	assets.Agents = readAgentFiles(filepath.Join(assetBase, "agents"))

	// themes/*.json, plugins/, tools/ — global-only in practice but read if present
	assets.ThemeFiles = readDirFiles(filepath.Join(assetBase, "themes"), ".json")
	assets.PluginFiles = readAllFiles(filepath.Join(assetBase, "plugins"))
	assets.ToolFiles = readAllFiles(filepath.Join(assetBase, "tools"))

	// Resolve @path references in exported markdown against refBase.
	refs := resolveAtRefs(collectMarkdownContents(assets), refBase)
	assets.ReferencedFiles = refs.Resolved
	assets.UnresolvedRefs = refs.Unresolved

	return assets, nil
}

// collectMarkdownContents gathers all markdown strings from the assets so that
// @-reference scanning can run over all of them in one pass.
func collectMarkdownContents(a *OpenCodeAssets) []string {
	var contents []string
	if a.AgentsMD != "" {
		contents = append(contents, a.AgentsMD)
	}
	for _, s := range a.Skills {
		contents = append(contents, s.Content)
	}
	for _, c := range a.CustomCommands {
		contents = append(contents, c.Content)
	}
	for _, ag := range a.Agents {
		contents = append(contents, ag.Content)
	}
	return contents
}

// parseTUIConfig converts a raw tui.json map into a TUIConfig struct.
func parseTUIConfig(raw map[string]any) *TUIConfig {
	cfg := &TUIConfig{}
	if v, ok := raw["theme"].(string); ok {
		cfg.Theme = v
	}
	if v, ok := raw["scroll_speed"].(float64); ok {
		cfg.ScrollSpeed = v
	}
	if v, ok := raw["scroll_acceleration"].(map[string]any); ok {
		cfg.ScrollAcceleration = v
	}
	if v, ok := raw["diff_style"].(string); ok {
		cfg.DiffStyle = v
	}
	if kb, ok := raw["keybinds"].(map[string]any); ok {
		keybinds := make(map[string]string, len(kb))
		for k, v := range kb {
			if s, ok := v.(string); ok {
				keybinds[k] = s
			}
		}
		if len(keybinds) > 0 {
			cfg.Keybinds = keybinds
		}
	}
	if cfg.Theme == "" && cfg.ScrollSpeed == 0 && cfg.ScrollAcceleration == nil &&
		cfg.DiffStyle == "" && len(cfg.Keybinds) == 0 {
		return nil
	}
	return cfg
}

// readExtraSkillFiles returns all files inside skillDir that are NOT SKILL.md.
func readExtraSkillFiles(skillDir string) []FileEntry {
	var result []FileEntry
	_ = filepath.WalkDir(skillDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(skillDir, path)
		if rel == "SKILL.md" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		result = append(result, FileEntry{
			Path:    filepath.ToSlash(rel),
			Content: string(data),
		})
		return nil
	})
	return result
}

// readMarkdownFilesRecursive reads every *.md file under dir recursively.
// The CommandEntry name is the slash-separated relative path without .md suffix.
func readMarkdownFilesRecursive(dir string, rel string) []CommandEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return nil
	}
	var result []CommandEntry
	for _, e := range entries {
		entryRel := e.Name()
		if rel != "" {
			entryRel = rel + "/" + e.Name()
		}
		if e.IsDir() {
			result = append(result, readMarkdownFilesRecursive(filepath.Join(dir, e.Name()), entryRel)...)
			continue
		}
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		result = append(result, CommandEntry{Name: strings.TrimSuffix(entryRel, ".md"), Content: string(data)})
	}
	return result
}

// readAgentFiles reads every *.md file in dir and returns AgentEntry slices.
func readAgentFiles(dir string) []AgentEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return nil
	}
	var result []AgentEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		result = append(result, AgentEntry{Name: strings.TrimSuffix(e.Name(), ".md"), Content: string(data)})
	}
	return result
}

// readDirFiles reads all files with the given extension in dir (non-recursive).
func readDirFiles(dir, ext string) []FileEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return nil
	}
	var result []FileEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ext) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		result = append(result, FileEntry{Path: e.Name(), Content: string(data)})
	}
	return result
}

// readAllFiles reads all files under dir recursively (any extension).
func readAllFiles(dir string) []FileEntry {
	var result []FileEntry
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		result = append(result, FileEntry{Path: filepath.ToSlash(rel), Content: string(data)})
		return nil
	})
	return result
}
