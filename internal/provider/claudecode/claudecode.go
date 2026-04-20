package claudecode

import (
	"github.com/castai/kimchi/internal/tools"
)

// Env returns environment variables for launching Claude Code with Kimchi configuration.
// Unlike OpenCode and Codex, Claude Code doesn't need a managed config directory
// because it reads configuration from ~/.claude/settings.json. However, for the
// wrapper launch, we inject the same env vars that would be in settings.json.
func Env(apiKey string) map[string]string {
	return map[string]string{
		"ANTHROPIC_BASE_URL":                     tools.AnthropicBaseURL(),
		"ANTHROPIC_API_KEY":                      apiKey,
		"CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS": "1",
	}
}
