package recipe

import (
	"strings"

	"github.com/castai/kimchi/internal/tools"
)

// ExportOptions carries the user's choices from the TUI wizard.
type ExportOptions struct {
	Name        string
	Author      string
	Description string
	UseCase     string

	IncludeAgentsMD       bool
	IncludeSkills         bool
	IncludeCustomCommands bool
	IncludeAgents         bool
}

// Build assembles a Recipe from OpenCode assets and the user's export options.
// It never includes API keys.
func Build(assets *OpenCodeAssets, opts ExportOptions) (*Recipe, error) {
	// Determine the active model slug from cfg["model"] which looks like "kimchi/kimi-k2.5".
	modelSlug := tools.MainModel.Slug
	if raw, ok := assets.Config["model"].(string); ok && raw != "" {
		parts := strings.SplitN(raw, "/", 2)
		if len(parts) == 2 {
			modelSlug = parts[1]
		} else {
			modelSlug = raw
		}
	}

	provider := buildProvider(assets.Config)

	ocCfg := &OpenCodeConfig{
		Provider:   provider,
		Model:      tools.ProviderName + "/" + modelSlug,
		Compaction: buildCompaction(assets.Config),
	}

	if opts.IncludeAgentsMD {
		ocCfg.AgentsMD = assets.AgentsMD
	}
	if opts.IncludeSkills {
		ocCfg.Skills = assets.Skills
	}
	if opts.IncludeCustomCommands {
		ocCfg.CustomCommands = assets.CustomCommands
	}
	if opts.IncludeAgents {
		ocCfg.Agents = assets.Agents
	}

	r := &Recipe{
		Name:        opts.Name,
		Author:      opts.Author,
		Description: opts.Description,
		Model:       modelSlug,
		UseCase:     opts.UseCase,
		Version:     "1",
		Tools: ToolsMap{
			OpenCode: ocCfg,
		},
	}

	return r, nil
}

// buildProvider extracts the kimchi provider block from the raw config map,
// constructing an OpenCodeProvider struct without any secret fields.
func buildProvider(cfg map[string]any) OpenCodeProvider {
	// Defaults from the known kimchi provider values.
	p := OpenCodeProvider{
		Name: "Kimchi by Cast AI",
		NPM:  "@ai-sdk/openai-compatible",
		Options: OpenCodeProviderOptions{
			BaseURL:      tools.BaseURL,
			LitellmProxy: true,
		},
		Models: buildDefaultModels(),
	}

	providers, ok := cfg["provider"].(map[string]any)
	if !ok {
		return p
	}
	kimchi, ok := providers[tools.ProviderName].(map[string]any)
	if !ok {
		return p
	}

	if name, ok := kimchi["name"].(string); ok && name != "" {
		p.Name = name
	}
	if npm, ok := kimchi["npm"].(string); ok && npm != "" {
		p.NPM = npm
	}
	if opts, ok := kimchi["options"].(map[string]any); ok {
		if u, ok := opts["baseURL"].(string); ok && u != "" {
			p.Options.BaseURL = u
		}
		if lp, ok := opts["litellmProxy"].(bool); ok {
			p.Options.LitellmProxy = lp
		}
		// apiKey is deliberately not read here
	}
	if models, ok := kimchi["models"].(map[string]any); ok {
		p.Models = parseModels(models)
	}

	return p
}

func buildDefaultModels() map[string]ModelDef {
	return map[string]ModelDef{
		tools.MainModel.Slug: {
			Name:      tools.MainModel.Slug,
			ToolCall:  tools.MainModel.GetToolCall(),
			Reasoning: tools.MainModel.GetReasoning(),
			Limit:     ModelLimit{Context: tools.MainModel.GetContextWindow(), Output: tools.MainModel.GetMaxOutputTokens()},
		},
		tools.CodingModel.Slug: {
			Name:      tools.CodingModel.Slug,
			ToolCall:  tools.CodingModel.GetToolCall(),
			Reasoning: tools.CodingModel.GetReasoning(),
			Limit:     ModelLimit{Context: tools.CodingModel.GetContextWindow(), Output: tools.CodingModel.GetMaxOutputTokens()},
		},
		tools.SubModel.Slug: {
			Name:     tools.SubModel.Slug,
			ToolCall: tools.SubModel.GetToolCall(),
			Limit:    ModelLimit{Context: tools.SubModel.GetContextWindow(), Output: tools.SubModel.GetMaxOutputTokens()},
		},
	}
}

func parseModels(raw map[string]any) map[string]ModelDef {
	result := make(map[string]ModelDef, len(raw))
	for slug, v := range raw {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		def := ModelDef{}
		if name, ok := m["name"].(string); ok {
			def.Name = name
		}
		if tc, ok := m["tool_call"].(bool); ok {
			def.ToolCall = tc
		}
		if r, ok := m["reasoning"].(bool); ok {
			def.Reasoning = r
		}
		if limit, ok := m["limit"].(map[string]any); ok {
			if ctx, ok := toInt(limit["context"]); ok {
				def.Limit.Context = ctx
			}
			if out, ok := toInt(limit["output"]); ok {
				def.Limit.Output = out
			}
		}
		result[slug] = def
	}
	return result
}

func buildCompaction(cfg map[string]any) CompactionConfig {
	c := CompactionConfig{Auto: true}
	if comp, ok := cfg["compaction"].(map[string]any); ok {
		if auto, ok := comp["auto"].(bool); ok {
			c.Auto = auto
		}
	}
	return c
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}
