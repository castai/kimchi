package tools

type limits struct {
	contextWindow   int
	maxOutputTokens int
}

type model struct {
	slug            string
	displayName     string
	description     string
	toolCall        bool
	reasoning       bool
	supportsImages  bool
	inputModalities []string
	limits          limits
}

var (
	reasoningModel = model{
		slug:            "glm-5-fp8",
		displayName:     "GLM-5 FP8",
		description:     "Reasoning model for planning, analysis, and complex problem solving.",
		toolCall:        true,
		reasoning:       true,
		inputModalities: []string{"text"},
		limits:          limits{contextWindow: 202752, maxOutputTokens: 32768},
	}
	codingModel = model{
		slug:            "minimax-m2.5",
		displayName:     "MiniMax M2.5",
		description:     "Coding model for writing, refactoring, and debugging code.",
		toolCall:        true,
		inputModalities: []string{"text"},
		limits:          limits{contextWindow: 196608, maxOutputTokens: 32768},
	}
	imageModel = model{
		slug:            "kimi-k2.5",
		displayName:     "Kimi K2.5",
		description:     "Multi-modal model for image processing and code generation.",
		toolCall:        true,
		supportsImages:  true,
		inputModalities: []string{"text", "image"},
		limits:          limits{contextWindow: 262144, maxOutputTokens: 32768},
	}

	allModels = []model{reasoningModel, codingModel, imageModel}
)
