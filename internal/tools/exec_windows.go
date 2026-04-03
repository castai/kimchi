//go:build windows

package tools

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
)

// ExecTool launches the binary as a child process on Windows (syscall.Exec is
// not available). The child inherits stdio and the exit code is propagated.
func ExecTool(binary string, args []string, env map[string]string) error {
	binaryPath, err := exec.LookPath(binary)
	if err != nil {
		return fmt.Errorf("%s is not installed or not in PATH", binary)
	}

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
		return fmt.Errorf("start %s: %w", binary, err)
	}

	go func() {
		for range sigCh {
			_ = cmd.Process.Signal(os.Interrupt)
		}
	}()

	err = cmd.Wait()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &ExitError{Code: exitErr.ExitCode()}
		}
		return fmt.Errorf("run %s: %w", binary, err)
	}

	return nil
}

// RunTool launches the binary as a child process with env overrides, forwards
// signals, waits for it to exit, then calls cleanup.
// Returns an *ExitError with the child's exit code so the caller can propagate it.
func RunTool(binary string, args []string, env map[string]string, cleanup func()) error {
	binaryPath, err := exec.LookPath(binary)
	if err != nil {
		return fmt.Errorf("%s is not installed or not in PATH", binary)
	}

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
		return fmt.Errorf("start %s: %w", binary, err)
	}

	go func() {
		for range sigCh {
			_ = cmd.Process.Signal(os.Interrupt)
		}
	}()

	err = cmd.Wait()

	if cleanup != nil {
		cleanup()
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &ExitError{Code: exitErr.ExitCode()}
		}
		return fmt.Errorf("run %s: %w", binary, err)
	}

	return nil
}
