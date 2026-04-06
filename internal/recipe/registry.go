package recipe

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/castai/kimchi/internal/cookbook"
)

// RecipeRef is a lightweight reference to a recipe in a local cookbook.
type RecipeRef struct {
	Name     string
	Version  string
	Cookbook string // cookbook name
	Author   string
	Tools    string // comma-joined list of supported tool names
	Path     string // absolute path to recipe.yaml
}

// ResolveSource parses a source string and returns the matching recipe.
//
// Supported formats:
//   - ./path/to/recipe.yaml  or  /abs/path  → read from file
//   - name@version           → match by name and exact version
//   - author/name            → match by cookbook name / recipe name
//   - name                   → match by recipe name (first match across cookbooks)
func ResolveSource(source string) (*Recipe, error) {
	if isFilePath(source) {
		return ReadFromFile(source)
	}
	ref, err := FindRecipe(source)
	if err != nil {
		return nil, err
	}
	return ReadFromFile(ref.Path)
}

// FindRecipe locates a recipe in the registered cookbooks by source string.
func FindRecipe(source string) (*RecipeRef, error) {
	name, version, cookbookName := parseSource(source)

	cookbooks, err := cookbook.Load()
	if err != nil {
		return nil, fmt.Errorf("load cookbooks: %w", err)
	}
	if len(cookbooks) == 0 {
		return nil, fmt.Errorf("no cookbooks registered — use `kimchi cookbook add <url>` first")
	}

	for _, cb := range cookbooks {
		if cookbookName != "" && cb.Name != cookbookName {
			continue
		}
		recipesDir := filepath.Join(cb.Path, "recipes")
		entries, err := os.ReadDir(recipesDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || !strings.EqualFold(e.Name(), name) {
				continue
			}
			p := filepath.Join(recipesDir, e.Name(), "recipe.yaml")
			r, err := readHeaderOnly(p)
			if err != nil {
				continue
			}
			if version != "" && r.Version != version {
				continue
			}
			return &RecipeRef{
				Name:     r.Name,
				Version:  r.Version,
				Cookbook: cb.Name,
				Author:   r.Author,
				Tools:    strings.Join(r.Tools.SupportedToolNames(), ", "),
				Path:     p,
			}, nil
		}
	}
	return nil, fmt.Errorf("recipe %q not found in any registered cookbook", source)
}

// ListAll returns all recipes found across all registered cookbooks.
func ListAll() ([]RecipeRef, error) {
	cookbooks, err := cookbook.Load()
	if err != nil {
		return nil, fmt.Errorf("load cookbooks: %w", err)
	}
	var refs []RecipeRef
	for _, cb := range cookbooks {
		found, err := listCookbook(cb)
		if err != nil {
			continue
		}
		refs = append(refs, found...)
	}
	return refs, nil
}

// Search returns recipes whose name, description, or tags match query.
func Search(query string) ([]RecipeRef, error) {
	all, err := ListAll()
	if err != nil {
		return nil, err
	}
	lower := strings.ToLower(query)
	var results []RecipeRef
	for _, ref := range all {
		if matchesQuery(ref, lower) {
			results = append(results, ref)
		}
	}
	return results, nil
}

func listCookbook(cb cookbook.Cookbook) ([]RecipeRef, error) {
	recipesDir := filepath.Join(cb.Path, "recipes")
	entries, err := os.ReadDir(recipesDir)
	if err != nil {
		return nil, err
	}
	var refs []RecipeRef
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(recipesDir, e.Name(), "recipe.yaml")
		r, err := readHeaderOnly(p)
		if err != nil {
			continue
		}
		refs = append(refs, RecipeRef{
			Name:     r.Name,
			Version:  r.Version,
			Cookbook: cb.Name,
			Author:   r.Author,
			Tools:    strings.Join(r.Tools.SupportedToolNames(), ", "),
			Path:     p,
		})
	}
	return refs, nil
}

func matchesQuery(ref RecipeRef, lower string) bool {
	if strings.Contains(strings.ToLower(ref.Name), lower) {
		return true
	}
	// Load full recipe for description / tags search
	r, err := readHeaderOnly(ref.Path)
	if err != nil {
		return false
	}
	if strings.Contains(strings.ToLower(r.Description), lower) {
		return true
	}
	for _, t := range r.Tags {
		if strings.Contains(strings.ToLower(t), lower) {
			return true
		}
	}
	return false
}

// readHeaderOnly reads only the top-level recipe fields (no tool config) for speed.
func readHeaderOnly(path string) (*Recipe, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Recipe
	if err := yaml.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// isFilePath reports whether source looks like a file system path.
func isFilePath(source string) bool {
	if strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../") || strings.HasPrefix(source, "/") {
		return true
	}
	if strings.HasSuffix(source, ".yaml") || strings.HasSuffix(source, ".yml") {
		return true
	}
	return false
}

// parseSource splits "name", "author/name", "name@version", "author/name@version"
// into (name, version, cookbookName).
func parseSource(source string) (name, version, cookbookName string) {
	// Split off @version
	if idx := strings.LastIndex(source, "@"); idx >= 0 {
		version = source[idx+1:]
		source = source[:idx]
	}
	// Split author/name
	if idx := strings.Index(source, "/"); idx >= 0 {
		cookbookName = source[:idx]
		name = source[idx+1:]
	} else {
		name = source
	}
	return
}
