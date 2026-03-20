package tools

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/castai/kimchi/internal/config"
)

const codexConfigPath = "~/.codex/config.toml"

func init() {
	register(Tool{
		ID:          ToolCodex,
		Name:        "Codex",
		Description: "OpenAI coding CLI",
		ConfigPath:  codexConfigPath,
		BinaryName:  "codex",
		IsInstalled: detectBinary("codex"),
		Write:       writeCodex,
	})
}

func writeCodex(scope config.ConfigScope) error {
	apiKey, err := config.GetAPIKey()
	if err != nil {
		return fmt.Errorf("get API key: %w", err)
	}
	if apiKey == "" {
		return fmt.Errorf("API key not configured")
	}

	envPath, err := config.ScopePaths(scope, "~/.codex/.env")
	if err != nil {
		return fmt.Errorf("get .env path: %w", err)
	}

	existingEnv, err := os.ReadFile(envPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read existing .env: %w", err)
	}

	envLines := parseEnvFile(string(existingEnv))
	envLines["OPENAI_BASE_URL"] = baseURL + "/v1/"
	envLines["OPENAI_API_KEY"] = apiKey
	envLines["OPENAI_MODEL"] = codingModel

	newEnvContent := formatEnvFile(envLines)
	if err := config.WriteFile(envPath, []byte(newEnvContent)); err != nil {
		return fmt.Errorf("write .env: %w", err)
	}

	configPath, err := config.ScopePaths(scope, codexConfigPath)
	if err != nil {
		return fmt.Errorf("get config.toml path: %w", err)
	}
	configContent := fmt.Sprintf(`[openai]
base_url = "%s/v1"
api_key = "%s"

[execution]
model = "%s"
`, baseURL, apiKey, codingModel)

	if err := config.WriteFile(configPath, []byte(configContent)); err != nil {
		return fmt.Errorf("write config.toml: %w", err)
	}

	instructionsPath, err := config.ScopePaths(scope, "~/.codex/AGENTS.md")
	if err != nil {
		return fmt.Errorf("get AGENTS.md path: %w", err)
	}
	instructions := `# Cast AI Configuration

This project is configured to use Cast AI's open-source models:
- glm-5-fp8 for reasoning/planning
- minimax-m2.5 for coding/execution

The API key and base URL are set in .codex/.env
`

	if err := config.WriteFile(instructionsPath, []byte(instructions)); err != nil {
		return fmt.Errorf("write AGENTS.md: %w", err)
	}

	return nil
}

func parseEnvFile(content string) map[string]string {
	result := make(map[string]string)
	lines := []byte(content)
	for len(lines) > 0 {
		line := lines
		idx := bytes.IndexByte(lines, '\n')
		if idx >= 0 {
			line = lines[:idx]
			lines = lines[idx+1:]
		} else {
			lines = nil
		}

		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] == '#' {
			continue
		}

		eqIdx := bytes.IndexByte(line, '=')
		if eqIdx > 0 {
			key := string(bytes.TrimSpace(line[:eqIdx]))
			value := string(bytes.TrimSpace(line[eqIdx+1:]))
			if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[0] == value[len(value)-1] {
				value = value[1 : len(value)-1]
			}
			result[key] = value
		}
	}
	return result
}

func formatEnvFile(env map[string]string) string {
	var buf strings.Builder
	for key, value := range env {
		buf.WriteString(key)
		buf.WriteString("=")
		if strings.ContainsAny(value, " \t\n\"'") {
			buf.WriteString("\"")
			buf.WriteString(strings.ReplaceAll(value, "\"", "\\\""))
			buf.WriteString("\"")
		} else {
			buf.WriteString(value)
		}
		buf.WriteString("\n")
	}
	return buf.String()
}
