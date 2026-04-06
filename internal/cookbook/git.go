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

// Push pushes commits (but not tags) to origin. Returns (false, nil) when the
// caller should fall back to a GitHub fork/PR flow. Tags are intentionally
// excluded so they are only pushed after a successful direct push via PushTag.
func Push(dir string) (hasAccess bool, err error) {
	out, err := run(dir, "git", "push")
	if err != nil {
		if isAuthError(out) {
			return false, nil
		}
		return false, fmt.Errorf("git push: %s", out)
	}
	return true, nil
}

// PushTag pushes a single named tag to origin.
func PushTag(dir, tag string) error {
	out, err := run(dir, "git", "push", "origin", tag)
	if err != nil {
		return fmt.Errorf("git push tag %s: %s", tag, out)
	}
	return nil
}

// CreateBranch creates and checks out a new branch.
func CreateBranch(dir, branch string) error {
	out, err := run(dir, "git", "checkout", "-b", branch)
	if err != nil {
		return fmt.Errorf("git checkout -b %s: %s", branch, out)
	}
	return nil
}

// PushBranch pushes a named branch to origin. Force-push is used because
// kimchi-managed branches may already exist from a previous interrupted push.
func PushBranch(dir, branch string) (hasAccess bool, err error) {
	out, err := run(dir, "git", "push", "-u", "origin", branch, "--force")
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

// HasUnpushedCommits reports whether the local branch is ahead of its remote tracking branch.
func HasUnpushedCommits(dir string) bool {
	out, err := run(dir, "git", "log", "@{u}..HEAD", "--oneline")
	return err == nil && strings.TrimSpace(out) != ""
}

// SyncToRemote fetches from origin and hard-resets the working tree to
// origin's default branch. Safe to call on kimchi-managed cookbook clones
// where local divergence is always from a previous interrupted push.
func SyncToRemote(dir string) error {
	return syncToRef(dir, "origin")
}

// SyncForkToUpstream fetches from the "upstream" remote and hard-resets the
// working tree to it, so fork branches are always based on the latest upstream.
func SyncForkToUpstream(dir string) error {
	return syncToRef(dir, "upstream")
}

func syncToRef(dir, remote string) error {
	if out, err := run(dir, "git", "fetch", remote); err != nil {
		return fmt.Errorf("git fetch %s: %s", remote, out)
	}
	// Resolve the remote HEAD to handle repos with non-main default branches.
	out, err := run(dir, "git", "rev-parse", "--abbrev-ref", remote+"/HEAD")
	if err != nil {
		out = remote + "/main"
	}
	ref := strings.TrimSpace(out)
	if out2, err := run(dir, "git", "reset", "--hard", ref); err != nil {
		return fmt.Errorf("git reset --hard %s: %s", ref, out2)
	}
	return nil
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
		// Protected branch — must go through a pull request.
		"protected branch",
		"gh006",
		"changes must be made through a pull request",
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
