package codex

import (
	"github.com/castai/kimchi/internal/tools"
)

// Env returns the environment variables needed to run Codex with Cast AI
// configuration. The apiKey is injected as KIMCHI_API_KEY, which the kimchi
// provider definition in ~/.codex/config.toml reads via env_key.
func Env(apiKey string) map[string]string {
	return map[string]string{
		tools.APIKeyEnv: apiKey,
	}
}
