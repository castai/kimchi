package recipe

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ForkOptions controls how `recipe fork` behaves.
type ForkOptions struct {
	// Source is a file path, "name", "cookbook/name", or "name@version".
	Source string
	// NewName overrides the recipe name in the forked copy.
	// If empty the original name is kept.
	NewName string
	// OutputPath is where the forked recipe yaml is written.
	// If empty it defaults to "<name>.yaml" in the current directory.
	OutputPath string
}

// Fork downloads (or reads) a recipe, marks it as forked, and writes the result locally.
func Fork(opts ForkOptions) (string, error) {
	var r *Recipe
	var err error

	if isFilePath(opts.Source) {
		r, err = ReadFromFile(opts.Source)
	} else {
		r, err = ResolveSource(opts.Source)
	}
	if err != nil {
		return "", fmt.Errorf("resolve recipe: %w", err)
	}

	// Record where this came from
	r.ForkedFrom = &ForkedFrom{
		Author:   r.Author,
		Cookbook: r.Cookbook,
		Version:  r.Version,
	}

	// Apply overrides
	if opts.NewName != "" {
		if opts.NewName != r.Name {
			fmt.Fprintf(os.Stderr,
				"warning: renaming recipe changes the forked_from lineage — the original was %q\n", r.Name)
		}
		r.Name = opts.NewName
	}

	// Reset fields for the fork
	r.Version = "0.1.0"
	r.Cookbook = "" // resolved on first push
	now := time.Now().UTC().Format(time.RFC3339)
	r.CreatedAt = now
	r.UpdatedAt = now

	// Determine output path
	out := opts.OutputPath
	if out == "" {
		out = sanitizeFilename(r.Name) + ".yaml"
	}

	if err := WriteYAML(out, r); err != nil {
		return "", fmt.Errorf("write fork: %w", err)
	}

	return filepath.Abs(out)
}

func sanitizeFilename(name string) string {
	return strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '-'
		}
		return r
	}, name)
}
