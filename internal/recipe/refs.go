package recipe

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// atRefPattern matches @token where the token looks like a file path:
// it must contain a '/' or a '.' to distinguish file refs from agent @mentions.
var atRefPattern = regexp.MustCompile(`@([\w.\-/]+\.[\w]+|[\w.\-]+/[\w.\-/]+)`)

// RefsResult holds the outcome of resolving @path references in markdown content.
type RefsResult struct {
	// Resolved contains files that were found inside the opencode config dir.
	Resolved []FileEntry
	// Unresolved contains reference strings that could not be embedded —
	// either they point outside the config directory or the file does not exist
	// there. These are likely project-level paths that the LLM will read at
	// runtime but cannot be bundled into the recipe.
	Unresolved []string
}

// resolveAtRefs scans markdown content for @path references, attempts to
// resolve each one against baseDir, and returns a RefsResult. Only files
// strictly inside baseDir are included — references that traverse outside
// (e.g. @../../etc/passwd) are reported as unresolved.
// Duplicate paths are deduplicated.
func resolveAtRefs(contents []string, baseDir string) RefsResult {
	// Ensure baseDir is absolute and clean so prefix checks are reliable.
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return RefsResult{}
	}
	// Guarantee a trailing separator so a directory named "opencode-extra" is
	// not mistaken for being inside "opencode".
	baseDirPrefix := absBase + string(filepath.Separator)

	seen := make(map[string]struct{})
	var res RefsResult

	for _, content := range contents {
		for _, match := range atRefPattern.FindAllStringSubmatch(content, -1) {
			ref := match[1]
			if _, already := seen[ref]; already {
				continue
			}
			seen[ref] = struct{}{}

			abs := filepath.Join(absBase, filepath.FromSlash(ref))

			// Reject any reference that resolves outside the config directory.
			if !strings.HasPrefix(abs, baseDirPrefix) {
				res.Unresolved = append(res.Unresolved, ref)
				continue
			}

			data, err := os.ReadFile(abs)
			if err != nil {
				// File not found in config dir — likely a project-level ref.
				res.Unresolved = append(res.Unresolved, ref)
				continue
			}
			res.Resolved = append(res.Resolved, FileEntry{
				Path:    ref, // keep as slash-separated relative path
				Content: string(data),
			})
		}
	}
	return res
}

// filterURLInstructions returns only the URL entries from the raw instructions
// slice (strings that start with "http://" or "https://").
// Local paths and glob patterns are machine-specific and not portable.
func filterURLInstructions(cfg map[string]any) []string {
	raw, ok := cfg["instructions"].([]any)
	if !ok {
		return nil
	}
	var urls []string
	for _, item := range raw {
		s, ok := item.(string)
		if !ok {
			continue
		}
		if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
			urls = append(urls, s)
		}
	}
	return urls
}
