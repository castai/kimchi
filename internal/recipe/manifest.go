package recipe

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/castai/kimchi/internal/tools"
)

// RecipeManifest records every file written by an install so they can be
// removed cleanly before a re-install.
type RecipeManifest struct {
	RecipeName  string       `json:"recipe_name"`
	Tool        tools.ToolID `json:"tool"`
	InstalledAt time.Time    `json:"installed_at"`
	AssetFiles  []string     `json:"asset_files"` // absolute paths of written files (not opencode.json)
}

func manifestPath(tool tools.ToolID, recipeName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kimchi", "manifests", string(tool), recipeName+".json"), nil
}

// LoadManifest returns the manifest for (tool, recipeName), or nil if absent.
func LoadManifest(tool tools.ToolID, recipeName string) (*RecipeManifest, error) {
	p, err := manifestPath(tool, recipeName)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m RecipeManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &m, nil
}

// SaveManifest persists a manifest. Dirs 0700, file 0600.
func SaveManifest(m *RecipeManifest) error {
	p, err := manifestPath(m.Tool, m.RecipeName)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}

// DeleteManifest removes the manifest for (tool, recipeName). No-op if absent.
func DeleteManifest(tool tools.ToolID, recipeName string) error {
	p, err := manifestPath(tool, recipeName)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// clearAllManifestsForTool removes all manifest files for a tool.
// Called when restoring a baseline slot.
func clearAllManifestsForTool(tool tools.ToolID) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".kimchi", "manifests", string(tool))
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
	return nil
}

// UninstallByManifest deletes all asset files listed in the manifest (best-effort).
func UninstallByManifest(tool tools.ToolID, recipeName string) error {
	m, err := LoadManifest(tool, recipeName)
	if err != nil || m == nil {
		return err
	}
	for _, p := range m.AssetFiles {
		_ = os.Remove(p) // best-effort
	}
	return nil
}
