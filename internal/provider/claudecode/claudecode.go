package claudecode

const anthropicBaseURL = "https://llm.kimchi.dev/anthropic"

// Env returns environment variables for launching Claude Code with Kimchi configuration.
// Unlike OpenCode and Codex, Claude Code doesn't need a managed config directory
// because it reads configuration from ~/.claude/settings.json. However, for the
// wrapper launch, we inject the same env vars that would be in settings.json.
func Env(apiKey string) map[string]string {
	return map[string]string{
		"ANTHROPIC_BASE_URL":                     anthropicBaseURL,
		"ANTHROPIC_API_KEY":                      apiKey,
		"CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS": "1",
	}
}

// InjectInto sets the Claude Code environment variables into an existing map[string]any.
func InjectInto(env map[string]any, baseURL, apiKey string) {
	env["ANTHROPIC_BASE_URL"] = baseURL
	env["ANTHROPIC_API_KEY"] = apiKey
	env["CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS"] = "1"
}
