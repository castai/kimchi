package tools

// ModelLimits holds the token limits for a model.
type ModelLimits struct {
	ContextWindow   int
	MaxOutputTokens int
}

// Model describes a single AI model available through the Kimchi inference endpoint.
type Model struct {
	Slug            string
	DisplayName     string
	Description     string
	ToolCall        bool
	Reasoning       bool
	SupportsImages  bool
	InputModalities []string
	Limits          ModelLimits
}

// ModelConfig groups the three role-assigned models used to configure tools,
// plus the full ordered list returned by the API.
type ModelConfig struct {
	Main   Model
	Coding Model
	Sub    Model
	All    []Model
}

// BuildModelConfig constructs a ModelConfig from a fetched model list and
// optional pre-saved role slug assignments. If a slug is empty or not found in
// the list, positional fallback is used (models[0], models[1], models[2]).
func BuildModelConfig(models []Model, mainSlug, codingSlug, subSlug string) ModelConfig {
	cfg := ModelConfig{All: models}

	if len(models) == 0 {
		return cfg
	}

	bySlug := make(map[string]Model, len(models))
	for _, m := range models {
		bySlug[m.Slug] = m
	}

	if m, ok := bySlug[mainSlug]; ok {
		cfg.Main = m
	}
	if m, ok := bySlug[codingSlug]; ok {
		cfg.Coding = m
	}
	if m, ok := bySlug[subSlug]; ok {
		cfg.Sub = m
	}

	// Fill any unset roles positionally.
	if cfg.Main.Slug == "" {
		cfg.Main = models[0]
	}
	if cfg.Coding.Slug == "" {
		if len(models) > 1 {
			cfg.Coding = models[1]
		} else {
			cfg.Coding = cfg.Main
		}
	}
	if cfg.Sub.Slug == "" {
		if len(models) > 2 {
			cfg.Sub = models[2]
		} else {
			cfg.Sub = cfg.Coding
		}
	}

	return cfg
}
