package update

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/Masterminds/semver/v3"
)

var semverRe = regexp.MustCompile(`v?\d+\.\d+\.\d+`)

// NewHarnessWorkflow returns a Workflow pre-configured for harness install/update.
// Caller-provided opts override the defaults.
func NewHarnessWorkflow(opts ...WorkflowOpt) *Workflow {
	defaults := []WorkflowOpt{
		WithExecutablePathFn(ResolveHarnessPath),
		WithCurrentVersionFn(HarnessCurrentVersion),
	}
	return NewWorkflow(kimchiDevRepo, append(defaults, opts...)...)
}

// CheckHarnessUpdate checks whether a harness install or update is available.
func CheckHarnessUpdate(ctx context.Context, opts ...WorkflowOpt) (*UpdateStatus, error) {
	w := NewHarnessWorkflow(append(opts, WithDryRun())...)
	res, err := w.Run(ctx)
	if err != nil {
		return nil, err
	}

	return &UpdateStatus{
		DisplayName:    "coding harness",
		CurrentVersion: res.InstalledVersion,
		LatestVersion:  res.AvailableVersion,
		HasUpdate:      res.HasUpdate(),
	}, nil
}

// HarnessCurrentVersion returns the installed harness version, or nil if the
// harness is not installed.
func HarnessCurrentVersion(ctx context.Context) (*semver.Version, error) {
	path, err := ResolveHarnessPath()
	if err != nil {
		return nil, err
	}
	if !HarnessInstalled(path) {
		return nil, nil
	}
	verStr, err := harnessVersion(ctx, path)
	if err != nil {
		return nil, err
	}
	return semver.NewVersion(verStr)
}

// HarnessPathInDir returns the harness binary path within the given directory.
func HarnessPathInDir(dir string) string {
	return filepath.Join(dir, kimchiDevRepo.Binary)
}

// ResolveHarnessPath derives the harness binary path from the kimchi executable's
// resolved directory. For example, if kimchi is at /usr/local/bin/kimchi, this
// returns /usr/local/bin/kimchi_code.
func ResolveHarnessPath() (string, error) {
	execPath, err := ResolveExecutablePath()
	if err != nil {
		return "", fmt.Errorf("resolve harness path: %w", err)
	}
	return HarnessPathInDir(filepath.Dir(execPath)), nil
}

// HarnessInstalled reports whether the harness binary exists at the given path.
func HarnessInstalled(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// harnessVersion runs the harness binary with "version" and parses the semver
// string from its output.
func harnessVersion(ctx context.Context, path string) (string, error) {
	out, err := exec.CommandContext(ctx, path, "version").Output()
	if err != nil {
		return "", fmt.Errorf("run %s version: %w", filepath.Base(path), err)
	}

	match := semverRe.Find(out)
	if match == nil {
		return "", fmt.Errorf("no semver found in output: %q", string(out))
	}

	v := string(match)
	if v[0] != 'v' {
		v = "v" + v
	}

	if _, err := semver.NewVersion(v); err != nil {
		return "", fmt.Errorf("invalid semver %q: %w", v, err)
	}

	return v, nil
}
