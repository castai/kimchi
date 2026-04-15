package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const modelsMetadataURL = proxyBaseURL + "/v1/models/metadata"

// apiModelMetadata mirrors the JSON shape returned by the proxy.
type apiModelMetadata struct {
	Slug            string          `json:"slug"`
	DisplayName     string          `json:"display_name"`
	Description     string          `json:"description"`
	ToolCall        bool            `json:"tool_call"`
	Reasoning       bool            `json:"reasoning"`
	SupportsImages  bool            `json:"supports_images"`
	InputModalities []string        `json:"input_modalities"`
	Limits          apiModelLimits  `json:"limits"`
}

type apiModelLimits struct {
	ContextWindow   int `json:"context_window"`
	MaxOutputTokens int `json:"max_output_tokens"`
}

type apiModelsMetadataResponse struct {
	Models []apiModelMetadata `json:"models"`
}

// FetchModels calls GET /v1/models/metadata?include_in_cli=true and returns the
// ordered list of models available for CLI tooling. Returns a non-nil error on
// any HTTP or decoding failure — callers must not proceed without a valid list.
func FetchModels(ctx context.Context, apiKey string) ([]Model, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsMetadataURL+"?include_in_cli=true", nil)
	if err != nil {
		return nil, fmt.Errorf("build models request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch models: unexpected HTTP %d", resp.StatusCode)
	}

	var body apiModelsMetadataResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode models response: %w", err)
	}

	if len(body.Models) == 0 {
		return nil, fmt.Errorf("API returned no CLI models")
	}

	models := make([]Model, 0, len(body.Models))
	for _, m := range body.Models {
		models = append(models, Model{
			Slug:            m.Slug,
			DisplayName:     m.DisplayName,
			Description:     m.Description,
			ToolCall:        m.ToolCall,
			Reasoning:       m.Reasoning,
			SupportsImages:  m.SupportsImages,
			InputModalities: m.InputModalities,
			Limits: ModelLimits{
				ContextWindow:   m.Limits.ContextWindow,
				MaxOutputTokens: m.Limits.MaxOutputTokens,
			},
		})
	}

	return models, nil
}
