package tools

type ModelLimits struct {
	ContextWindow   int
	MaxOutputTokens int
}

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

var (
	ReasoningModel = Model{
		Slug:            "glm-5-fp8",
		DisplayName:     "GLM-5 FP8",
		Description:     "Reasoning model for planning, analysis, and complex problem solving.",
		ToolCall:        true,
		Reasoning:       true,
		InputModalities: []string{"text"},
		Limits:          ModelLimits{ContextWindow: 202752, MaxOutputTokens: 32768},
	}
	CodingModel = Model{
		Slug:            "minimax-m2.5",
		DisplayName:     "MiniMax M2.5",
		Description:     "Coding model for writing, refactoring, and debugging code.",
		ToolCall:        true,
		InputModalities: []string{"text"},
		Limits:          ModelLimits{ContextWindow: 196608, MaxOutputTokens: 32768},
	}
	ImageModel = Model{
		Slug:            "kimi-k2.5",
		DisplayName:     "Kimi K2.5",
		Description:     "Multi-modal model for image processing and code generation.",
		ToolCall:        true,
		SupportsImages:  true,
		InputModalities: []string{"text", "image"},
		Limits:          ModelLimits{ContextWindow: 262144, MaxOutputTokens: 32768},
	}

	allModels = []Model{ReasoningModel, CodingModel, ImageModel}
)
