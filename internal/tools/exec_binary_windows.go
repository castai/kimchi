//go:build windows

package tools

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
)

// ExecBinary launches the binary at the given absolute path as a child process
// on Windows (syscall.Exec is not available). The child inherits stdio and the
// exit code is propagated. Unlike ExecTool it does not search PATH.
func ExecBinary(binaryPath string, args []string, env map[string]string) error {
	cmd := exec.Command(binaryPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = MergeEnv(env)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer func() {
		signal.Stop(sigCh)
		close(sigCh)
	}()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", binaryPath, err)
	}

	go func() {
		for range sigCh {
			_ = cmd.Process.Signal(os.Interrupt)
		}
	}()

	err := cmd.Wait()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &ExitError{Code: exitErr.ExitCode()}
		}
		return fmt.Errorf("run %s: %w", binaryPath, err)
	}

	return nil
}
