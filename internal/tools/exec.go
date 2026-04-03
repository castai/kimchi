//go:build !windows

package tools

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
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

	fullArgs := append([]string{binaryPath}, args...)
	return syscall.Exec(binaryPath, fullArgs, MergeEnv(env))
}

// RunTool launches the binary as a child process with env overrides, forwards
// signals, waits for it to exit, then calls cleanup. This keeps the kimchi
// process alive so cleanup can run (unlike ExecTool which replaces the process).
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
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer func() {
		signal.Stop(sigCh)
		close(sigCh)
	}()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", binary, err)
	}

	go func() {
		for sig := range sigCh {
			_ = cmd.Process.Signal(sig)
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
