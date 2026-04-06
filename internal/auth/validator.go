package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type validator struct {
	client *http.Client
}

func NewValidator(client *http.Client) Validator {
	if client == nil {
		client = &http.Client{
			Timeout: RequestTimeout * time.Second,
		}
	}
	return &validator{client: client}
}

func (v *validator) ValidateAPIKey(ctx context.Context, apiKey string) (ValidateResult, error) {
	if apiKey == "" {
		return ValidateResult{
			Valid: false,
			Error: "API key is required",
			Suggestions: []string{
				"Get your API key at https://app.kimchi.dev",
				"Set it via KIMCHI_API_KEY environment variable or run 'kimchi'",
			},
		}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ValidationEndpoint, nil)
	if err != nil {
		return ValidateResult{}, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	req.Header.Set("Accept", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return ValidateResult{
			Valid: false,
			Error: "Network error: unable to reach Cast AI API",
			Suggestions: []string{
				"Check your internet connection",
				"Verify you can reach https://api.cast.ai",
				"Try again in a few moments",
			},
		}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return ValidateResult{Valid: true}, nil
	case http.StatusUnauthorized:
		return ValidateResult{
			Valid: false,
			Error: "Invalid API key",
			Suggestions: []string{
				"Verify your API key at https://app.kimchi.dev",
				"Ensure the key has not been revoked",
				"Check for typos or extra whitespace",
			},
		}, nil
	case http.StatusForbidden:
		return ValidateResult{
			Valid: false,
			Error: "API key lacks required permissions",
			Suggestions: []string{
				"Verify your API key has the required scopes at https://app.kimchi.dev",
				"Contact support if the issue persists",
			},
		}, nil
	default:
		return ValidateResult{
			Valid: false,
			Error: fmt.Sprintf("API returned status %d", resp.StatusCode),
			Suggestions: []string{
				"Try again in a few moments",
				"Check status.cast.ai for service status",
				"Contact support if the issue persists",
			},
		}, nil
	}
}
