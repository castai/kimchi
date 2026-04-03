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
	Config         map[string]any
	AgentsMD       string
	Skills         []SkillEntry
	CustomCommands []CommandEntry
	Agents         []AgentEntry
}

// ReadOpenCodeAssets reads all exportable assets from the user's global
// OpenCode configuration directory (~/.config/opencode/).
func ReadOpenCodeAssets() (*OpenCodeAssets, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	base := filepath.Join(homeDir, ".config", "opencode")
	assets := &OpenCodeAssets{}

	// opencode.json / opencode.jsonc
	cfg, err := config.ReadJSON(filepath.Join(base, "opencode.json"))
	if err != nil {
		return nil, err
	}
	ScrubSecrets(cfg)
	assets.Config = cfg

	// AGENTS.md
	if data, err := os.ReadFile(filepath.Join(base, "AGENTS.md")); err == nil {
		assets.AgentsMD = string(data)
	}

	// skills/<name>/SKILL.md
	skillsDir := filepath.Join(base, "skills")
	if entries, err := os.ReadDir(skillsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			skillFile := filepath.Join(skillsDir, e.Name(), "SKILL.md")
			if data, err := os.ReadFile(skillFile); err == nil {
				assets.Skills = append(assets.Skills, SkillEntry{
					Name:    e.Name(),
					Content: string(data),
				})
			}
		}
	}

	// commands/*.md
	assets.CustomCommands = readMarkdownFiles(filepath.Join(base, "commands"))

	// agents/*.md
	assets.Agents = readAgentFiles(filepath.Join(base, "agents"))

	return assets, nil
}

// readMarkdownFiles reads every *.md file in dir and returns CommandEntry slices.
func readMarkdownFiles(dir string) []CommandEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return nil
	}
	var result []CommandEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		result = append(result, CommandEntry{Name: name, Content: string(data)})
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
		name := strings.TrimSuffix(e.Name(), ".md")
		result = append(result, AgentEntry{Name: name, Content: string(data)})
	}
	return result
}
