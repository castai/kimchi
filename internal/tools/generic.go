package tools

import (
	"fmt"

	"github.com/castai/kimchi/internal/config"
)

func init() {
	register(Tool{
		ID:          ToolGeneric,
		Name:        "Generic",
		Description: "Export env vars (stdout)",
		ConfigPath:  "",
		BinaryName:  "",
		IsInstalled: func() bool { return false },
		Write:       writeGeneric,
	})
}

func writeGeneric(scope config.ConfigScope, apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("API key not configured")
	}

	fmt.Printf("export %s=%s\n", APIKeyEnv, apiKey)
	fmt.Printf("export OPENAI_API_KEY=%s\n", apiKey)
	fmt.Printf("export OPENAI_BASE_URL=%s\n", baseURL)

	return nil
}
