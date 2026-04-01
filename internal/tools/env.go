package tools

import (
	"fmt"
	"os"
	"strings"
)

// MergeEnv builds an environment slice from the current process environment
// with the provided overrides applied.
func MergeEnv(env map[string]string) []string {
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
	return merged
}

// ExitError is returned by RunTool to propagate the child process exit code
// without calling os.Exit directly, allowing deferred cleanup to run.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit status %d", e.Code)
}
