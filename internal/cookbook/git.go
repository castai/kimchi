package cookbook

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Clone clones a remote repository to destDir.
func Clone(url, destDir string) error {
	if err := os.MkdirAll(filepath.Dir(destDir), 0755); err != nil {
		return err
	}
	out, err := run("", "git", "clone", url, destDir)
	if err != nil {
		return fmt.Errorf("git clone: %s", out)
	}
	return nil
}

// Pull fetches and fast-forwards the default branch.
func Pull(dir string) error {
	out, err := run(dir, "git", "pull", "--ff-only")
	if err != nil {
		return fmt.Errorf("git pull: %s", out)
	}
	return nil
}

// AddFiles stages the given file paths inside dir.
func AddFiles(dir string, paths []string) error {
	args := append([]string{"add", "--"}, paths...)
	out, err := run(dir, "git", args...)
	if err != nil {
		return fmt.Errorf("git add: %s", out)
	}
	return nil
}

// Commit creates a commit with the given message.
func Commit(dir, message string) error {
	out, err := run(dir, "git", "commit", "-m", message)
	if err != nil {
		return fmt.Errorf("git commit: %s", out)
	}
	return nil
}

// CreateTag creates an annotated tag. Annotated tags (unlike lightweight ones)
// are pushed by `git push --follow-tags`.
func CreateTag(dir, tag string) error {
	out, err := run(dir, "git", "tag", "-a", tag, "-m", tag)
	if err != nil {
		return fmt.Errorf("git tag %s: %s", tag, out)
	}
	return nil
}

// TagExists reports whether the given tag already exists in the repo.
func TagExists(dir, tag string) bool {
	out, err := run(dir, "git", "tag", "-l", tag)
	return err == nil && strings.TrimSpace(out) == tag
}

// Push pushes commits and tags to origin. Returns (false, nil) when auth fails
// (indicating the caller should fall back to a GitHub fork/PR flow).
func Push(dir string) (hasAccess bool, err error) {
	out, err := run(dir, "git", "push", "--follow-tags")
	if err != nil {
		if isAuthError(out) {
			return false, nil
		}
		return false, fmt.Errorf("git push: %s", out)
	}
	return true, nil
}

// CreateBranch creates and checks out a new branch.
func CreateBranch(dir, branch string) error {
	out, err := run(dir, "git", "checkout", "-b", branch)
	if err != nil {
		return fmt.Errorf("git checkout -b %s: %s", branch, out)
	}
	return nil
}

// PushBranch pushes a named branch to origin.
func PushBranch(dir, branch string) (hasAccess bool, err error) {
	out, err := run(dir, "git", "push", "-u", "origin", branch)
	if err != nil {
		if isAuthError(out) {
			return false, nil
		}
		return false, fmt.Errorf("git push branch: %s", out)
	}
	return true, nil
}

// AddRemote adds a named remote.
func AddRemote(dir, name, url string) error {
	out, err := run(dir, "git", "remote", "add", name, url)
	if err != nil {
		return fmt.Errorf("git remote add: %s", out)
	}
	return nil
}

// RemoteURL returns the fetch URL for the given remote name.
func RemoteURL(dir, remote string) (string, error) {
	out, err := run(dir, "git", "remote", "get-url", remote)
	if err != nil {
		return "", fmt.Errorf("git remote get-url: %s", out)
	}
	return strings.TrimSpace(out), nil
}

// IsRepo reports whether dir is inside a git repository.
func IsRepo(dir string) bool {
	_, err := run(dir, "git", "rev-parse", "--git-dir")
	return err == nil
}

// HasUncommitted reports whether there are uncommitted changes.
func HasUncommitted(dir string) bool {
	out, err := run(dir, "git", "status", "--porcelain")
	return err == nil && strings.TrimSpace(out) != ""
}

func isAuthError(output string) bool {
	lower := strings.ToLower(output)
	for _, s := range []string{
		"permission denied",
		"authentication failed",
		"could not read username",
		"invalid credentials",
		"403",
		"401",
		"repository not found",
		"remote: error: access denied",
	} {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

func run(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}
