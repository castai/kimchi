package recipe

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/castai/kimchi/internal/tools"
)

func installedPerToolPath(tool tools.ToolID) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kimchi", "installed", string(tool)+".json"), nil
}

// migrateInstalledIfNeeded moves ~/.kimchi/installed.json → per-tool files.
// Idempotent: renames the old file to .migrated after migration.
func migrateInstalledIfNeeded() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	legacyPath := filepath.Join(home, ".kimchi", "installed.json")
	data, err := os.ReadFile(legacyPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var list []InstalledRecipe
	if err := json.Unmarshal(data, &list); err != nil {
		return nil // corrupt legacy file; leave it alone
	}
	byTool := make(map[tools.ToolID][]InstalledRecipe)
	for _, r := range list {
		if r.Tool == "" {
			r.Tool = tools.ToolOpenCode
		}
		byTool[r.Tool] = append(byTool[r.Tool], r)
	}
	for toolID, recs := range byTool {
		if err := saveInstalledForTool(toolID, recs); err != nil {
			return err
		}
	}
	_ = os.Rename(legacyPath, legacyPath+".migrated")
	return nil
}

func loadInstalledForTool(tool tools.ToolID) ([]InstalledRecipe, error) {
	if err := migrateInstalledIfNeeded(); err != nil {
		return nil, err
	}
	p, err := installedPerToolPath(tool)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read installed: %w", err)
	}
	var list []InstalledRecipe
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parse installed: %w", err)
	}
	return list, nil
}

func saveInstalledForTool(tool tools.ToolID, list []InstalledRecipe) error {
	p, err := installedPerToolPath(tool)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}

// clearAllInstalledForTool wipes all installed records for a tool.
// Called when restoring a baseline slot (all recipes are gone).
func clearAllInstalledForTool(tool tools.ToolID) error {
	return saveInstalledForTool(tool, nil)
}

