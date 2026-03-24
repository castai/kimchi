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

	"github.com/minio/selfupdate"
)

func backupDir() (string, error) {
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appDir, "backups"), nil
}

// ApplyOption configures the behavior of Apply.
type ApplyOption func(*applyOptions)

type applyOptions struct {
	executablePath string
	progressWriter io.Writer
}

// WithExecutablePath sets the path of the executable to replace. If not set, os.Executable() is used.
func WithExecutablePath(path string) ApplyOption {
	return func(o *applyOptions) { o.executablePath = path }
}

// WithProgressWriter sets the writer where download progress is rendered.
func WithProgressWriter(w io.Writer) ApplyOption {
	return func(o *applyOptions) { o.progressWriter = w }
}

// Apply downloads the release for the given version, verifies the archive checksum,
// extracts the binary, and atomically replaces the current executable.
// It backs up the current binary and verifies the new one runs successfully.
func Apply(ctx context.Context, client GitHubClient, version string, opts ...ApplyOption) error {
	var o applyOptions
	for _, opt := range opts {
		opt(&o)
	}

	expectedChecksum, err := client.FetchChecksum(ctx, version)
	if err != nil {
		return fmt.Errorf("fetch checksum: %w", err)
	}

	binaryPath, err := downloadAndVerify(ctx, client, version, expectedChecksum, o.progressWriter)
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(filepath.Dir(binaryPath)) }()

	backupPath, err := prepareBackup(version)
	if err != nil {
		return fmt.Errorf("prepare backup: %w", err)
	}

	f, err := os.Open(binaryPath)
	if err != nil {
		return fmt.Errorf("open downloaded binary: %w", err)
	}
	defer func() { _ = f.Close() }()

	suOpts := selfupdate.Options{
		TargetPath:  o.executablePath,
		OldSavePath: backupPath,
	}
	if err := suOpts.CheckPermissions(); err != nil {
		return fmt.Errorf("permission denied: %w", err)
	}

	if err := selfupdate.Apply(f, suOpts); err != nil {
		if rerr := selfupdate.RollbackError(err); rerr != nil {
			return fmt.Errorf("update failed and rollback failed: %w (rollback: %v)", err, rerr)
		}
		return fmt.Errorf("update failed, restored previous version: %w", err)
	}

	// Resolve the path that was actually updated.
	verifyPath := o.executablePath
	if verifyPath == "" {
		verifyPath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("get executable path: %w", err)
		}
	}

	if err := verifyBinary(verifyPath); err != nil {
		backup, berr := os.Open(backupPath)
		if berr != nil {
			return fmt.Errorf("new binary broken and cannot restore: %w (restore: %v)", err, berr)
		}
		defer func() { _ = backup.Close() }()

		if rerr := selfupdate.Apply(backup, selfupdate.Options{TargetPath: o.executablePath}); rerr != nil {
			return fmt.Errorf("new binary broken and restore failed: %w (restore: %v)", err, rerr)
		}
		return fmt.Errorf("new binary verification failed, restored previous version: %w", err)
	}

	cleanOldBackups(backupPath)
	return nil
}

// downloadAndVerify downloads the archive, verifies its SHA256 checksum,
// then extracts the binary. Returns the path to the extracted binary.
func downloadAndVerify(ctx context.Context, client GitHubClient, version string, expectedChecksum []byte, progressWriter io.Writer) (string, error) {
	tmpDir, err := os.MkdirTemp("", "kimchi-update-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	archivePath := filepath.Join(tmpDir, assetName())
	if err := client.DownloadArchive(ctx, version, archivePath, progressWriter); err != nil {
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
	defer func() { _ = archiveFile.Close() }()

	binaryPath, err := extractBinary(archiveFile)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", fmt.Errorf("extract binary: %w", err)
	}

	// Clean up the archive, keep only the extracted binary's temp dir.
	_ = os.RemoveAll(tmpDir)
	return binaryPath, nil
}

func prepareBackup(version string) (string, error) {
	dir, err := backupDir()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create backup directory: %w", err)
	}

	return filepath.Join(dir, "kimchi-"+version), nil
}

// cleanOldBackups removes all backups except the one at keepPath.
func cleanOldBackups(keepPath string) {
	dir := filepath.Dir(keepPath)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if p != keepPath {
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

func extractBinary(r io.Reader) (string, error) {
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

		if hdr.Typeflag != tar.TypeReg || hdr.Name != "kimchi" {
			continue
		}

		tmpDir, err := os.MkdirTemp("", "kimchi-update-*")
		if err != nil {
			return "", err
		}

		outPath := filepath.Join(tmpDir, "kimchi")
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

	return "", fmt.Errorf("kimchi binary not found in archive")
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
