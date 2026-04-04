package recipe

import (
	"strings"
)

// SecretPlaceholderPrefix is the unique prefix used for all secret placeholders
// in exported recipes. The installer can detect any placeholder with:
//
//	strings.HasPrefix(value, recipe.SecretPlaceholderPrefix)
const SecretPlaceholderPrefix = "kimchi:secret:"

// placeholder returns a uniquely-prefixed placeholder string for a secret.
// Example: placeholder("openai", "apiKey") → "kimchi:secret:OPENAI_APIKEY"
func placeholder(parts ...string) string {
	upper := make([]string, len(parts))
	for i, p := range parts {
		upper[i] = strings.ToUpper(p)
	}
	return SecretPlaceholderPrefix + strings.Join(upper, "_")
}

// secretProviderKeys are option keys treated as secrets in provider configs.
var secretProviderKeys = []string{"apiKey", "api_key", "token", "secret"}

// ScrubSecrets replaces known secret fields in provider and MCP configs with
// placeholder strings (e.g. "kimchi:secret:OPENAI_APIKEY") so the exported
// recipe is safe to share. The installer detects placeholders via
// SecretPlaceholderPrefix and prompts the user to supply real values.
func ScrubSecrets(cfg map[string]any) map[string]any {
	scrubProviders(cfg)
	scrubMCP(cfg)
	return cfg
}

// scrubProviders replaces secret option values for every provider entry.
func scrubProviders(cfg map[string]any) {
	providers, ok := cfg["provider"].(map[string]any)
	if !ok {
		return
	}
	for name, v := range providers {
		prov, ok := v.(map[string]any)
		if !ok {
			continue
		}
		opts, ok := prov["options"].(map[string]any)
		if !ok {
			continue
		}
		for _, key := range secretProviderKeys {
			if _, exists := opts[key]; exists {
				opts[key] = placeholder(name, key)
			}
		}
	}
}

// scrubMCP replaces secrets in MCP server definitions:
//   - environment vars for local servers
//   - HTTP headers for remote servers
//   - OAuth client credentials for remote servers
func scrubMCP(cfg map[string]any) {
	mcp, ok := cfg["mcp"].(map[string]any)
	if !ok {
		return
	}
	for name, v := range mcp {
		server, ok := v.(map[string]any)
		if !ok {
			continue
		}

		// Local MCP: environment variables
		if env, ok := server["environment"].(map[string]any); ok {
			for key := range env {
				env[key] = placeholder("mcp", name, key)
			}
		}

		// Remote MCP: HTTP headers
		if headers, ok := server["headers"].(map[string]any); ok {
			for key := range headers {
				headers[key] = placeholder("mcp", name, "header", key)
			}
		}

		// Remote MCP: OAuth credentials
		if oauth, ok := server["oauth"].(map[string]any); ok {
			if _, ok := oauth["clientId"]; ok {
				oauth["clientId"] = placeholder("mcp", name, "oauth", "client", "id")
			}
			if _, ok := oauth["clientSecret"]; ok {
				oauth["clientSecret"] = placeholder("mcp", name, "oauth", "client", "secret")
			}
		}
	}
}
