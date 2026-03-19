package auth

import "context"

type ValidateResult struct {
	Valid       bool
	Error       string
	Suggestions []string
}

type Validator interface {
	ValidateAPIKey(ctx context.Context, apiKey string) (ValidateResult, error)
}

const (
	ValidationEndpoint = "https://api.cast.ai/v1/llm/openai/supported-providers"
	RequestTimeout     = 10
)
