package update

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// extractStructuredArchive extracts a tar.gz archive preserving its directory
// structure into a temporary directory. It verifies that bin/<binaryName> exists
// in the archive. Returns the root extraction directory. The caller must clean
// it up with os.RemoveAll.
func extractStructuredArchive(r io.Reader, binaryName string) (string, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return "", err
	}
	defer func() { _ = gz.Close() }()

	tmpDir, err := os.MkdirTemp("", "kimchi-update-*")
	if err != nil {
		return "", err
	}

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
		}
	}

	binaryPath := filepath.Join(tmpDir, "bin", binaryName)
	if _, err := os.Stat(binaryPath); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", fmt.Errorf("%s binary not found in archive", binaryName)
	}

	return tmpDir, nil
}
