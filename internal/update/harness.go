package update

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
)

// NewHarnessWorkflow returns a Workflow pre-configured for harness install/update.
// Caller-provided opts override the defaults.
func NewHarnessWorkflow(opts ...WorkflowOpt) *Workflow {
	defaults := []WorkflowOpt{
		WithExecutablePathFn(ResolveHarnessPath),
		WithCurrentVersionFn(HarnessCurrentVersion),
		WithDataDir(harnessDataDir()),
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
// harness is not installed. It reads the version from package.json in the XDG
// data directory, falling back to next to the binary for pre-split installs.
func HarnessCurrentVersion(_ context.Context) (*semver.Version, error) {
	path, err := ResolveHarnessPath()
	if err != nil {
		return nil, err
	}
	if !HarnessInstalled(path) {
		return nil, nil
	}
	pkgPath, err := resolveHarnessPackageJSON(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return nil, fmt.Errorf("read package.json: %w", err)
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("parse package.json: %w", err)
	}
	if pkg.Version == "" {
		return nil, nil
	}
	return semver.NewVersion(pkg.Version)
}

// resolveHarnessPackageJSON returns the path to package.json, checking the XDG
// data directory first and falling back to next to the binary for pre-split installs.
func resolveHarnessPackageJSON(binaryPath string) (string, error) {
	p := filepath.Join(harnessDataDir(), "package.json")
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	return filepath.Join(filepath.Dir(binaryPath), "package.json"), nil
}

// HarnessPathInDir returns the harness binary path within the given directory.
func HarnessPathInDir(dir string) string {
	return filepath.Join(dir, kimchiDevRepo.Binary)
}

// ResolveHarnessPath derives the harness binary path from the kimchi executable's
// resolved directory. For example, if kimchi is at /usr/local/bin/kimchi, this
// returns /usr/local/bin/kimchi-code.
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
