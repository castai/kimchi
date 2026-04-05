package cookbook

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	// builtinDefaultCookbookURL is compiled into the binary.
	// Override at runtime with KIMCHI_DEFAULT_COOKBOOK_URL.
	// Set KIMCHI_DEFAULT_COOKBOOK_URL="" to disable the default cookbook entirely.
	builtinDefaultCookbookURL = "https://github.com/castai/kimchi-cookbook.git"

	// DefaultCookbookName is the local name used for the default cookbook.
	DefaultCookbookName = "kimchi-cookbook"
)

// DefaultCookbookURL returns the URL of the default cookbook.
// It returns the value of KIMCHI_DEFAULT_COOKBOOK_URL if set (even if empty,
// which disables the default cookbook), otherwise the compiled-in URL.
func DefaultCookbookURL() string {
	if v, ok := os.LookupEnv("KIMCHI_DEFAULT_COOKBOOK_URL"); ok {
		return v
	}
	return builtinDefaultCookbookURL
}

// IsDefault reports whether name refers to the default cookbook.
func IsDefault(name string) bool {
	return name == DefaultCookbookName
}

// defaultCookbookPath returns the local clone path for the default cookbook.
func defaultCookbookPath() (string, error) {
	cloneDir, err := DefaultCloneDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cloneDir, DefaultCookbookName), nil
}

// EnsureDefault clones the default cookbook if it is not already present on
// disk. It is a no-op when:
//   - KIMCHI_DEFAULT_COOKBOOK_URL is set to an empty string (disabled)
//   - The clone directory already exists and looks like a git repository
//
// Clone errors are returned so the caller can decide whether to treat them as
// fatal.
func EnsureDefault(outW io.Writer) error {
	defURL := DefaultCookbookURL()
	if defURL == "" {
		return nil // explicitly disabled
	}

	defPath, err := defaultCookbookPath()
	if err != nil {
		return err
	}

	// Already cloned — nothing to do.
	if _, err := os.Stat(filepath.Join(defPath, ".git")); err == nil {
		return nil
	}

	fmt.Fprintf(outW, "==> Cloning default cookbook (%s)…\n", DefaultCookbookName)
	if err := os.MkdirAll(filepath.Dir(defPath), 0755); err != nil {
		return fmt.Errorf("create cookbook dir: %w", err)
	}
	return Clone(defURL, defPath)
}
