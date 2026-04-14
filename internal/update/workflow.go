package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/minio/selfupdate"

	"github.com/castai/kimchi/internal/version"
)

// WorkflowOpt configures a Workflow.
type WorkflowOpt func(w *Workflow)

// WorkflowResult holds the outcome of a workflow run.
type WorkflowResult struct {
	InstalledVersion *semver.Version // version that was installed before the workflow ran (nil = fresh install)
	AvailableVersion *semver.Version // latest version available from the release
	Updated          bool            // Updated is false for dry run or when no update was available
}

// FreshInstall reports whether this is a new installation (no previous version).
func (r *WorkflowResult) FreshInstall() bool {
	return r.InstalledVersion == nil
}

// HasUpdate reports whether a newer version is available (or a fresh install is needed).
func (r *WorkflowResult) HasUpdate() bool {
	return r.FreshInstall() || (r.AvailableVersion != nil && r.AvailableVersion.GreaterThan(r.InstalledVersion))
}

func WithDryRun() WorkflowOpt {
	return func(w *Workflow) { w.dryRun = true }
}

func WithSkipUpdateCache() WorkflowOpt {
	return func(w *Workflow) { w.skipCache = true }
}

// WithClient sets the GitHub client used for release lookups.
func WithClient(client GitHubClient) WorkflowOpt {
	return func(w *Workflow) { w.client = client }
}

// WithExecutablePathFn sets the function that resolves the target executable
// path. Defaults to ResolveExecutablePath.
func WithExecutablePathFn(fn func() (string, error)) WorkflowOpt {
	return func(w *Workflow) { w.getExecutablePathFn = fn }
}

// WithCurrentVersionFn sets the function that returns the currently installed
// version. Return nil when the binary is not installed (triggers a fresh install).
func WithCurrentVersionFn(fn func(ctx context.Context) (*semver.Version, error)) WorkflowOpt {
	return func(w *Workflow) { w.getCurrentVersionFn = fn }
}

// WithConfirmFn sets the function called when an update (or install) is
// available. It receives the current version (nil if not installed) and the
// latest version. Return true to proceed with the apply step.
func WithConfirmFn(fn func(current, latest *semver.Version) (bool, error)) WorkflowOpt {
	return func(w *Workflow) { w.confirmFn = fn }
}

// WithProgressFn sets a function that wraps the apply step with progress UI
// (e.g. a spinner). The function receives the tag being installed and a work
// closure that performs the actual apply.
func WithProgressFn(fn func(tag string, work func() error) error) WorkflowOpt {
	return func(w *Workflow) { w.progressFn = fn }
}

// Workflow is a self-contained update algorithm: check → confirm → apply.
type Workflow struct {
	client GitHubClient
	repo   Repo

	getExecutablePathFn func() (string, error)
	getCurrentVersionFn func(ctx context.Context) (*semver.Version, error)
	confirmFn           func(current, latest *semver.Version) (bool, error)
	progressFn          func(tag string, work func() error) error

	skipCache bool
	dryRun    bool
}

func NewWorkflow(repo Repo, opts ...WorkflowOpt) *Workflow {
	w := &Workflow{repo: repo}
	for _, opt := range opts {
		opt(w)
	}

	if w.getCurrentVersionFn == nil {
		w.getCurrentVersionFn = func(_ context.Context) (*semver.Version, error) {
			v, err := semver.NewVersion(version.Version)
			if err != nil {
				return nil, nil // "dev" or unparseable => treat as not-installed
			}
			return v, nil
		}
	}
	if w.confirmFn == nil {
		w.confirmFn = func(_, _ *semver.Version) (bool, error) {
			return true, nil
		}
	}
	if w.client == nil {
		w.client = NewGitHubClient()
	}
	if w.getExecutablePathFn == nil {
		w.getExecutablePathFn = ResolveExecutablePath
	}

	return w
}

// Run executes the full update workflow: check → confirm → apply.
//
// It returns a WorkflowResult describing what happened. The caller is
// responsible for presentation (messages, spinners, etc.).
//
// Error semantics:
//   - version check failure → returned as error
//   - no update available   → result with Updated=false, nil error
//   - dry run               → result with Updated=false, nil error
//   - confirm returns false → result with Updated=false, nil error
//   - apply failure         → returned as wrapped error
func (w *Workflow) Run(ctx context.Context) (*WorkflowResult, error) {
	// Step 1: Determine the currently installed version.
	current, err := w.getCurrentVersionFn(ctx)
	if err != nil {
		return nil, fmt.Errorf("get current version: %w", err)
	}

	// Step 2: Check for updates via GitHub (with optional cache).
	res, err := w.checkForUpdate(ctx)
	if err != nil {
		return nil, fmt.Errorf("check for updates: %w", err)
	}

	latest := res.version
	var hasUpdate bool
	if current == nil {
		// Not installed — always treat as needing install.
		hasUpdate = true
	} else {
		hasUpdate = latest.GreaterThan(current)
	}

	result := &WorkflowResult{
		InstalledVersion: current,
		AvailableVersion: latest,
	}

	if !hasUpdate {
		return result, nil
	}

	// Step 3: Dry run — report what would happen without applying.
	if w.dryRun {
		return result, nil
	}

	// Step 4: Check permissions before prompting the user.
	if _, err := w.getExecutablePathFn(); err != nil {
		return nil, fmt.Errorf("resolve executable path: %w", err)
	}

	// Step 5: Confirm with the user.
	if w.confirmFn != nil {
		ok, err := w.confirmFn(current, latest)
		if err != nil {
			return nil, fmt.Errorf("confirm: %w", err)
		}
		if !ok {
			return result, nil
		}
	}

	// Step 6: Apply the update.
	freshInstall := current == nil
	applyWork := func() error {
		return w.apply(ctx, res.tag, freshInstall)
	}
	if w.progressFn != nil {
		err = w.progressFn(res.tag, applyWork)
	} else {
		err = applyWork()
	}
	if err != nil {
		return nil, fmt.Errorf("apply: %w", err)
	}

	result.Updated = true
	return result, nil
}

type latestVersion struct {
	version    *semver.Version
	tag        string
	releaseURL string
}

// checkForUpdate fetches the latest release version, using a 24h cached state
// when fresh or querying the GitHub API when stale.
func (w *Workflow) checkForUpdate(ctx context.Context) (*latestVersion, error) {
	var latestTag, releaseURL string

	if !w.skipCache {
		rs, _ := loadRepoState(w.repo)
		if rs != nil && !rs.IsStale(time.Now()) && rs.LatestVersion != "" {
			latestTag = rs.LatestVersion
			releaseURL = rs.ReleaseURL
		}
	}

	if latestTag == "" {
		info, err := w.client.LatestRelease(ctx, w.repo)
		if err != nil {
			return nil, err
		}

		latestTag = info.TagName
		releaseURL = info.HTMLURL

		_ = saveRepoState(w.repo, &repoState{
			CheckedAt:     time.Now(),
			LatestVersion: latestTag,
			ReleaseURL:    releaseURL,
		})
	}

	lat, err := semver.NewVersion(latestTag)
	if err != nil {
		return nil, err
	}

	return &latestVersion{
		version:    lat,
		tag:        latestTag,
		releaseURL: releaseURL,
	}, nil
}

// checkPermissions verifies the current process can write to the given executable path.
func checkPermissions(executablePath string) error {
	opts := selfupdate.Options{TargetPath: executablePath}
	if err := opts.CheckPermissions(); err != nil {
		return fmt.Errorf("cannot update %s: permission denied (try running with sudo)", executablePath)
	}
	return nil
}

// apply downloads the release for the given version, verifies the archive checksum,
// extracts the binary, and atomically replaces the current executable.
func (w *Workflow) apply(ctx context.Context, tag string, freshInstall bool) error {
	executablePath, err := w.getExecutablePathFn()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	if err := checkPermissions(executablePath); err != nil {
		return err
	}

	expectedChecksum, err := w.client.FetchChecksum(ctx, w.repo, tag)
	if err != nil {
		return fmt.Errorf("fetch checksum: %w", err)
	}

	binaryPath, err := w.downloadAndVerify(ctx, tag, expectedChecksum)
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(filepath.Dir(binaryPath)) }()

	f, err := os.Open(binaryPath)
	if err != nil {
		return fmt.Errorf("open downloaded binary: %w", err)
	}
	defer func() { _ = f.Close() }()

	if freshInstall {
		return applyFreshInstall(f, executablePath)
	}
	return applyUpdate(f, w.repo.Binary, tag, executablePath)
}

// applyFreshInstall handles the case where the target binary does not yet exist.
// It copies the binary to a temp file in the target directory and atomically
// renames it into place. On verification failure the installed file is removed.
func applyFreshInstall(newBinary *os.File, targetPath string) error {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}

	tmpPath := targetPath + ".tmp"
	tmpFile, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	if _, err := io.Copy(tmpFile, newBinary); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("copy binary: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("install binary: %w", err)
	}

	if err := verifyBinary(targetPath); err != nil {
		_ = os.Remove(targetPath)
		return fmt.Errorf("binary verification failed, removed install: %w", err)
	}
	return nil
}

// applyUpdate handles the standard update case where an existing binary is being replaced.
// It backs up the current binary and restores it if verification fails.
func applyUpdate(newBinary *os.File, binaryName, version, executablePath string) error {
	backupPath, err := prepareBackup(binaryName, version)
	if err != nil {
		return fmt.Errorf("prepare backup: %w", err)
	}

	suOpts := selfupdate.Options{
		TargetPath:  executablePath,
		OldSavePath: backupPath,
	}
	if err := suOpts.CheckPermissions(); err != nil {
		return fmt.Errorf("permission denied: %w", err)
	}
	if err := selfupdate.Apply(newBinary, suOpts); err != nil {
		if rerr := selfupdate.RollbackError(err); rerr != nil {
			return fmt.Errorf("update failed and rollback failed: %w (rollback: %v)", err, rerr)
		}
		return fmt.Errorf("update failed, restored previous version: %w", err)
	}

	if err := verifyBinary(executablePath); err != nil {
		backup, berr := os.Open(backupPath)
		if berr != nil {
			return fmt.Errorf("new binary broken and cannot restore: %w (restore: %v)", err, berr)
		}
		defer func() { _ = backup.Close() }()

		if rerr := selfupdate.Apply(backup, selfupdate.Options{TargetPath: executablePath}); rerr != nil {
			return fmt.Errorf("new binary broken and restore failed: %w (restore: %v)", err, rerr)
		}
		return fmt.Errorf("new binary verification failed, restored previous version: %w", err)
	}

	cleanOldBackups(binaryName, backupPath)
	return nil
}

// downloadAndVerify downloads the archive, verifies its SHA256 checksum,
// then extracts the binary. Returns the path to the extracted binary.
func (w *Workflow) downloadAndVerify(ctx context.Context, version string, expectedChecksum []byte) (string, error) {
	tmpDir, err := os.MkdirTemp("", "kimchi-update-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	archivePath := filepath.Join(tmpDir, assetName(w.repo))
	if err := w.client.DownloadArchive(ctx, w.repo, version, archivePath); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", fmt.Errorf("download archive: %w", err)
	}

	if err := verifyChecksum(archivePath, expectedChecksum); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", err
	}

	archiveFile, err := os.Open(archivePath)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", err
	}

	// extractBinary creates its own temp directory for the output binary,
	// so binaryPath is independent of tmpDir and survives the cleanup below.
	binaryPath, err := extractBinary(archiveFile, w.repo.Binary)
	_ = archiveFile.Close()
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", fmt.Errorf("extract binary: %w", err)
	}

	_ = os.RemoveAll(tmpDir)
	return binaryPath, nil
}

func prepareBackup(binaryName, version string) (string, error) {
	dir, err := backupDir()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create backup directory: %w", err)
	}

	return filepath.Join(dir, binaryName+"-"+version), nil
}

// cleanOldBackups removes backups for the given binary except the one at keepPath.
// Backups for other binaries sharing the same directory are left untouched.
func cleanOldBackups(binaryName, keepPath string) {
	dir := filepath.Dir(keepPath)
	prefix := binaryName + "-"
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if p != keepPath && strings.HasPrefix(e.Name(), prefix) {
			_ = os.Remove(p)
		}
	}
}

func verifyBinary(path string) error {
	out, err := exec.Command(path, "version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("binary verification failed: %w (output: %s)", err, string(out))
	}
	return nil
}

func extractBinary(r io.Reader, binaryName string) (string, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return "", err
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		if hdr.Typeflag != tar.TypeReg || filepath.Base(hdr.Name) != binaryName {
			continue
		}

		tmpDir, err := os.MkdirTemp("", "kimchi-update-*")
		if err != nil {
			return "", err
		}

		outPath := filepath.Join(tmpDir, binaryName)
		out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY, 0755)
		if err != nil {
			_ = os.RemoveAll(tmpDir)
			return "", err
		}

		if _, err := io.Copy(out, tr); err != nil {
			_ = out.Close()
			_ = os.RemoveAll(tmpDir)
			return "", err
		}
		_ = out.Close()

		return outPath, nil
	}

	return "", fmt.Errorf("%s binary not found in archive", binaryName)
}

func verifyChecksum(path string, expected []byte) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file for checksum: %w", err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("compute checksum: %w", err)
	}

	actual := h.Sum(nil)
	if !bytes.Equal(actual, expected) {
		return fmt.Errorf("checksum mismatch: expected %x, got %x", expected, actual)
	}

	return nil
}
