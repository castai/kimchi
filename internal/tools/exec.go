//go:build !windows

package tools

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
)

// mergeEnv builds an environment slice from the current process environment
// with the provided overrides applied.
func mergeEnv(env map[string]string) []string {
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
	return syscall.Exec(binaryPath, fullArgs, mergeEnv(env))
}

// RunTool launches the binary as a child process with env overrides, forwards
// signals, waits for it to exit, then calls cleanup. This keeps the kimchi
// process alive so cleanup can run (unlike ExecTool which replaces the process).
// The child's exit code is propagated via os.Exit.
func RunTool(binary string, args []string, env map[string]string, cleanup func()) error {
	binaryPath, err := exec.LookPath(binary)
	if err != nil {
		return fmt.Errorf("%s is not installed or not in PATH", binary)
	}

	cmd := exec.Command(binaryPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = mergeEnv(env)

	// Forward signals to the child process.
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

	// Propagate exit code.
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("run %s: %w", binary, err)
	}

	os.Exit(0)
	return nil // unreachable
}
