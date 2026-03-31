//go:build !windows

package tools

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// ExecTool replaces the current process with the given binary, injecting
// the provided environment variables. All args are passed through verbatim.
// syscall.Exec is used for transparent process replacement so the child
// inherits terminal control and signal handling directly from the shell.
func ExecTool(binary string, args []string, env map[string]string) error {
	binaryPath, err := exec.LookPath(binary)
	if err != nil {
		return fmt.Errorf("%s is not installed or not in PATH", binary)
	}

	existing := os.Environ()
	merged := make([]string, 0, len(existing)+len(env))
	for _, e := range existing {
		key, _, _ := strings.Cut(e, "=")
		if _, override := env[key]; !override {
			merged = append(merged, e)
		}
	}
	for k, v := range env {
		merged = append(merged, k+"="+v)
	}

	fullArgs := append([]string{binaryPath}, args...)

	return syscall.Exec(binaryPath, fullArgs, merged)
}
