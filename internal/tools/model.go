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
	ReasoningModel = model{
		Slug:            "glm-5-fp8",
		displayName:     "GLM-5 FP8",
		description:     "Reasoning model for planning, analysis, and complex problem solving.",
		toolCall:        true,
		reasoning:       true,
		inputModalities: []string{"text"},
		limits:          limits{contextWindow: 202752, maxOutputTokens: 32768},
	}
	CodingModel = model{
		Slug:            "minimax-m2.5",
		displayName:     "MiniMax M2.5",
		description:     "Coding model for writing, refactoring, and debugging code.",
		toolCall:        true,
		inputModalities: []string{"text"},
		limits:          limits{contextWindow: 196608, maxOutputTokens: 32768},
	}
	ImageModel = model{
		Slug:            "kimi-k2.5",
		displayName:     "Kimi K2.5",
		description:     "Multi-modal model for image processing and code generation.",
		toolCall:        true,
		supportsImages:  true,
		inputModalities: []string{"text", "image"},
		limits:          limits{contextWindow: 262144, maxOutputTokens: 32768},
	}

	allModels = []model{ReasoningModel, CodingModel, ImageModel}
)
