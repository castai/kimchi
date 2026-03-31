//go:build windows

package tools

import "fmt"

// ExecTool is not supported on Windows.
func ExecTool(binary string, args []string, env map[string]string) error {
	return fmt.Errorf("kimchi wrapper commands are not supported on Windows")
}
