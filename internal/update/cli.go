package update

import (
	"context"
)

// NewCLIWorkflow returns a Workflow pre-configured for CLI self-update.
// Caller-provided opts override the defaults.
func NewCLIWorkflow(opts ...WorkflowOpt) *Workflow {
	defaults := []WorkflowOpt{
		WithExecutablePathFn(ResolveExecutablePath),
	}
	return NewWorkflow(kimchiRepo, append(defaults, opts...)...)
}

// CheckCLIUpdate checks whether a CLI update is available.
func CheckCLIUpdate(ctx context.Context, opts ...WorkflowOpt) (*UpdateStatus, error) {
	w := NewCLIWorkflow(append(opts, WithDryRun())...)
	res, err := w.Run(ctx)
	if err != nil {
		return nil, err
	}

	return &UpdateStatus{
		DisplayName:    "kimchi",
		CurrentVersion: res.InstalledVersion,
		LatestVersion:  res.AvailableVersion,
		HasUpdate:      res.HasUpdate(),
	}, nil
}

