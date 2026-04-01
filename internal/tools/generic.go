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

func writeGeneric(scope config.ConfigScope) error {
	apiKey, err := config.GetAPIKey()
	if err != nil {
		return fmt.Errorf("get API key: %w", err)
	}
	if apiKey == "" {
		return fmt.Errorf("API key not configured")
	}

	fmt.Printf("export CASTAI_API_KEY=%s\n", apiKey)
	fmt.Printf("export OPENAI_API_KEY=%s\n", apiKey)
	fmt.Printf("export OPENAI_BASE_URL=%s\n", baseURL)

	return nil
}
