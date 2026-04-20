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
	"k8s.io/klog/v2"

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

// WithDataDir sets the directory for supporting files (package.json, theme/).
// When set, the workflow uses structured archive extraction and places
// supporting files in this directory instead of next to the binary.
func WithDataDir(path string) WorkflowOpt {
	return func(w *Workflow) { w.dataDir = path }
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

	dataDir   string
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
// extracts all contents, and installs them to the target directory.
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

	extractDir, err := w.downloadAndVerify(ctx, tag, expectedChecksum)
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(extractDir) }()

	var binaryPath string
	if w.dataDir != "" {
		binaryPath = filepath.Join(extractDir, "bin", w.repo.Binary)
	} else {
		binaryPath = filepath.Join(extractDir, w.repo.Binary)
	}
	f, err := os.Open(binaryPath)
	if err != nil {
		return fmt.Errorf("open downloaded binary: %w", err)
	}
	defer func() { _ = f.Close() }()

	var backupPath string
	if freshInstall {
		klog.V(1).InfoS("installing binary", "repo", w.repo.Name, "path", executablePath)
		if err := applyFreshInstall(f, executablePath); err != nil {
			return err
		}
	} else {
		klog.V(1).InfoS("updating binary", "repo", w.repo.Name, "path", executablePath)
		var err error
		backupPath, err = applyUpdate(f, w.repo.Binary, tag, executablePath)
		if err != nil {
			return err
		}
	}
	klog.V(1).InfoS("binary installed", "repo", w.repo.Name, "path", executablePath)

	// Copy supporting files before verification — the binary may depend on them.
	var supportSrc, supportDst string
	if w.dataDir != "" {
		supportSrc = filepath.Join(extractDir, "share", "kimchi")
		supportDst = w.dataDir
	} else {
		supportSrc = extractDir
		supportDst = filepath.Dir(executablePath)
	}
	if err := copySupportingFiles(supportSrc, supportDst, w.repo.Binary); err != nil {
		return err
	}
	klog.V(1).InfoS("supporting files copied", "repo", w.repo.Name, "targetDir", supportDst)

	klog.V(1).InfoS("verifying binary", "repo", w.repo.Name, "path", executablePath)
	if err := verifyBinary(executablePath); err != nil {
		klog.ErrorS(err, "binary verification failed", "repo", w.repo.Name, "path", executablePath)
		if freshInstall {
			_ = os.Remove(executablePath)
			klog.V(1).InfoS("removed failed fresh install", "path", executablePath)
		} else if backupPath != "" {
			if backup, berr := os.Open(backupPath); berr == nil {
				_ = selfupdate.Apply(backup, selfupdate.Options{TargetPath: executablePath})
				_ = backup.Close()
				klog.V(1).InfoS("rolled back to previous version", "repo", w.repo.Name, "backup", backupPath)
			}
		}
		return fmt.Errorf("binary verification failed: %w", err)
	}
	return nil
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
	return nil
}

// applyUpdate handles the standard update case where an existing binary is being replaced.
// It backs up the current binary and returns the backup path so the caller can
// roll back if post-install verification fails.
func applyUpdate(newBinary *os.File, binaryName, version, executablePath string) (backupPath string, err error) {
	backupPath, err = prepareBackup(binaryName, version)
	if err != nil {
		return "", fmt.Errorf("prepare backup: %w", err)
	}

	suOpts := selfupdate.Options{
		TargetPath:  executablePath,
		OldSavePath: backupPath,
	}
	if err := suOpts.CheckPermissions(); err != nil {
		return "", fmt.Errorf("permission denied: %w", err)
	}
	if err := selfupdate.Apply(newBinary, suOpts); err != nil {
		if rerr := selfupdate.RollbackError(err); rerr != nil {
			return "", fmt.Errorf("update failed and rollback failed: %w (rollback: %v)", err, rerr)
		}
		return "", fmt.Errorf("update failed, restored previous version: %w", err)
	}

	cleanOldBackups(binaryName, backupPath)
	return backupPath, nil
}

// downloadAndVerify downloads the archive, verifies its SHA256 checksum,
// and extracts all contents. Returns the path to the temporary directory
// containing the extracted files. The caller must clean up the directory.
func (w *Workflow) downloadAndVerify(ctx context.Context, version string, expectedChecksum []byte) (string, error) {
	tmpDir, err := os.MkdirTemp("", "kimchi-update-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	archivePath := filepath.Join(tmpDir, assetName(w.repo))
	klog.V(1).InfoS("downloading release archive", "repo", w.repo.Name, "version", version)
	if err := w.client.DownloadArchive(ctx, w.repo, version, archivePath); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", fmt.Errorf("download archive: %w", err)
	}
	klog.V(1).InfoS("download complete", "repo", w.repo.Name, "version", version)

	if err := verifyChecksum(archivePath, expectedChecksum); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", err
	}
	klog.V(1).InfoS("checksum verified", "repo", w.repo.Name, "version", version)

	archiveFile, err := os.Open(archivePath)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", err
	}

	klog.V(1).InfoS("extracting archive", "repo", w.repo.Name, "version", version)
	var extractDir string
	if w.dataDir != "" {
		extractDir, err = extractStructuredArchive(archiveFile, w.repo.Binary)
	} else {
		extractDir, err = extractArchive(archiveFile, w.repo.Binary)
	}
	_ = archiveFile.Close()
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", fmt.Errorf("extract archive: %w", err)
	}
	klog.V(1).InfoS("extraction complete", "repo", w.repo.Name, "version", version)

	_ = os.RemoveAll(tmpDir)
	return extractDir, nil
}

func prepareBackup(binaryName, version string) (string, error) {
	dir := backupDir()
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
	out, err := exec.Command(path, "--version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("binary verification failed: %w (output: %s)", err, string(out))
	}
	return nil
}

// extractArchive extracts all files from a tar.gz archive into a temporary
// directory and returns the directory path. It verifies that the expected
// binary is present in the archive. The caller must clean up the directory.
func extractArchive(r io.Reader, binaryName string) (string, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return "", err
	}
	defer func() { _ = gz.Close() }()

	tmpDir, err := os.MkdirTemp("", "kimchi-update-*")
	if err != nil {
		return "", err
	}

	foundBinary := false
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			_ = os.RemoveAll(tmpDir)
			return "", err
		}

		// Sanitize path to prevent directory traversal.
		clean := filepath.Clean(hdr.Name)
		if strings.HasPrefix(clean, "..") {
			continue
		}
		outPath := filepath.Join(tmpDir, clean)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(outPath, 0755); err != nil {
				_ = os.RemoveAll(tmpDir)
				return "", err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
				_ = os.RemoveAll(tmpDir)
				return "", err
			}
			out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY, hdr.FileInfo().Mode())
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

			if filepath.Base(hdr.Name) == binaryName {
				foundBinary = true
			}
		}
	}

	if !foundBinary {
		_ = os.RemoveAll(tmpDir)
		return "", fmt.Errorf("%s binary not found in archive", binaryName)
	}

	return tmpDir, nil
}

// copySupportingFiles copies all files and directories from srcDir to dstDir,
// skipping entries that match skipName.
//
// For flat archives (CLI), all files including the binary live in one directory;
// skipName is the binary name so it isn't copied twice.
// For structured archives (harness), the binary is under bin/ and supporting
// files are under share/; skipName has no effect since the binary isn't present
// in the source directory.
func copySupportingFiles(srcDir, dstDir, skipName string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("read extract directory: %w", err)
	}

	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("create destination directory %s: %w", dstDir, err)
	}

	for _, entry := range entries {
		if entry.Name() == skipName {
			continue
		}
		src := filepath.Join(srcDir, entry.Name())
		dst := filepath.Join(dstDir, entry.Name())

		if entry.IsDir() {
			if err := copyDir(src, dst); err != nil {
				return err
			}
		} else {
			if err := copyFile(src, dst); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer func() { _ = in.Close() }()

	info, err := in.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s: %w", dst, err)
	}
	return nil
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", dst, err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read directory %s: %w", src, err)
	}

	for _, entry := range entries {
		s := filepath.Join(src, entry.Name())
		d := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(s, d); err != nil {
				return err
			}
		} else {
			if err := copyFile(s, d); err != nil {
				return err
			}
		}
	}
	return nil
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
