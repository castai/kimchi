//go:build !windows

package tools

import "syscall"

// ExecBinary replaces the current process with the binary at the given absolute
// path, injecting the provided environment variables. Unlike ExecTool it does
// not search PATH — the caller is responsible for providing a resolved path.
func ExecBinary(binaryPath string, args []string, env map[string]string) error {
	fullArgs := append([]string{binaryPath}, args...)
	return syscall.Exec(binaryPath, fullArgs, MergeEnv(env))
}
