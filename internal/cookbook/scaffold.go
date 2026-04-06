package cookbook

import (
	"fmt"
	"os"
	"path/filepath"
)

const cookbookYAML = `name: %s
`

const readmeMD = `# %s

A kimchi recipe cookbook.

## Structure

` + "```" + `
recipes/
  <recipe-name>/
    recipe.yaml
` + "```" + `
`

// Scaffold writes a minimal cookbook structure into dir, commits, and pushes.
// It expects dir to already be a git repo (cloned from the remote).
func Scaffold(dir, name string) error {
	// Create recipes/ directory
	if err := os.MkdirAll(filepath.Join(dir, "recipes"), 0755); err != nil {
		return err
	}

	// .kimchi/cookbook.yaml
	kimchiDir := filepath.Join(dir, ".kimchi")
	if err := os.MkdirAll(kimchiDir, 0755); err != nil {
		return err
	}
	cbYAML := fmt.Sprintf(cookbookYAML, name)
	if err := os.WriteFile(filepath.Join(kimchiDir, "cookbook.yaml"), []byte(cbYAML), 0644); err != nil {
		return err
	}

	// README.md (only if it doesn't already exist)
	readmePath := filepath.Join(dir, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		if err := os.WriteFile(readmePath, []byte(fmt.Sprintf(readmeMD, name)), 0644); err != nil {
			return err
		}
	}

	// Commit and push scaffold
	if !HasUncommitted(dir) {
		return nil // nothing to commit (e.g. remote already had these files)
	}
	if err := AddFiles(dir, []string{".kimchi", "recipes", "README.md"}); err != nil {
		return err
	}
	if err := Commit(dir, "Initial cookbook scaffold"); err != nil {
		return err
	}
	out, err := run(dir, "git", "push")
	if err != nil {
		return fmt.Errorf("git push scaffold: %s", out)
	}
	return nil
}
