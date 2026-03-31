package tools

type limits struct {
	contextWindow   int
	maxOutputTokens int
}

type model struct {
	Slug            string
	displayName     string
	description     string
	toolCall        bool
	reasoning       bool
	supportsImages  bool
	inputModalities []string
	limits          limits
}

var (
	// MainModel is the primary model for reasoning, planning, code generation, and image processing.
	MainModel = model{
		Slug:            "kimi-k2.5",
		displayName:     "Kimi K2.5",
		description:     "Primary model for reasoning, planning, code generation, and image processing.",
		toolCall:        true,
		reasoning:       true,
		supportsImages:  true,
		inputModalities: []string{"text", "image"},
		limits:          limits{contextWindow: 262144, maxOutputTokens: 32768},
	}
	// CodingModel is the coding subagent used where tools require a fixed model value for code tasks.
	CodingModel = model{
		Slug:            "glm-5-fp8",
		displayName:     "GLM-5 FP8",
		description:     "Coding subagent for writing, refactoring, and debugging code.",
		toolCall:        true,
		reasoning:       true,
		inputModalities: []string{"text"},
		limits:          limits{contextWindow: 202752, maxOutputTokens: 32768},
	}
	// SubModel is the secondary subagent available across all tool installations.
	SubModel = model{
		Slug:            "minimax-m2.5",
		displayName:     "MiniMax M2.5",
		description:     "Secondary subagent for code generation and debugging.",
		toolCall:        true,
		inputModalities: []string{"text"},
		limits:          limits{contextWindow: 196608, maxOutputTokens: 32768},
	}

	allModels = []model{MainModel, CodingModel, SubModel}
)
