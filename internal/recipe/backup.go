package recipe

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/castai/kimchi/internal/tools"
)

// BackupMeta is written as meta.json inside every backup slot.
type BackupMeta struct {
	Tool              tools.ToolID `json:"tool"`
	RecipeName        string       `json:"recipe_name,omitempty"`         // empty for baseline
	RecipeVersion     string       `json:"recipe_version,omitempty"`      // version at time of capture
	RecipeCookbook    string       `json:"recipe_cookbook,omitempty"`     // cookbook at time of capture
	CapturedAt        time.Time    `json:"captured_at"`
	Files             []string     `json:"files"` // paths relative to $HOME
}

// BackupSlot describes one backup directory available for restore.
type BackupSlot struct {
	Tool       tools.ToolID
	RecipeName string // empty = baseline
	CapturedAt time.Time
	Dir        string // absolute path to the backup directory
	Meta       BackupMeta
}

func backupsRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kimchi", "backups"), nil
}

func backupBaselineDir(tool tools.ToolID) (string, error) {
	root, err := backupsRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, string(tool), "baseline"), nil
}

func backupSlotDir(tool tools.ToolID, recipeName string) (string, error) {
	root, err := backupsRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, string(tool), recipeName), nil
}

// EnsureBaseline captures the current state of filesToCapture into the
// baseline slot for this tool. Does nothing if a baseline already exists.
// Returns the absolute paths that were actually backed up (nil when the
// baseline already existed and no files were captured).
func EnsureBaseline(tool tools.ToolID, filesToCapture []string) ([]string, error) {
	dir, err := backupBaselineDir(tool)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(dir, "meta.json")); err == nil {
		return nil, nil // baseline already exists
	}
	return captureFilesToDir(dir, BackupMeta{Tool: tool}, filesToCapture)
}

// snapshotWithMeta is like Snapshot but also stores version and cookbook in meta.json.
// Returns the absolute paths that were actually backed up.
func snapshotWithMeta(tool tools.ToolID, recipeName, version, cookbook string, filesToCapture []string) ([]string, error) {
	dir, err := backupSlotDir(tool, recipeName)
	if err != nil {
		return nil, err
	}
	_ = os.RemoveAll(dir) // remove stale slot
	return captureFilesToDir(dir, BackupMeta{
		Tool:           tool,
		RecipeName:     recipeName,
		RecipeVersion:  version,
		RecipeCookbook: cookbook,
		CapturedAt:     time.Now(),
	}, filesToCapture)
}

// SnapshotCurrentlyInstalled captures the current on-disk state of every recipe
// already installed for tool into per-recipe backup slots. This preserves the
// pre-upgrade state so users can roll back to it. Should be called right before
// installing a new recipe, after EnsureBaseline.
// Returns the union of all absolute paths that were actually backed up.
func SnapshotCurrentlyInstalled(tool tools.ToolID) ([]string, error) {
	installed, err := loadInstalledForTool(tool)
	if err != nil || len(installed) == 0 {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	var allBacked []string
	for _, rec := range installed {
		m, _ := LoadManifest(tool, rec.Name)
		var filesToCapture []string
		if m != nil {
			filesToCapture = append(filesToCapture, m.AssetFiles...)
		}
		// opencode.json is excluded from manifests (merge target) but should be
		// included in the backup so the full config state is preserved.
		filesToCapture = append(filesToCapture, filepath.Join(home, ".config", "opencode", "opencode.json"))
		backed, err := snapshotWithMeta(tool, rec.Name, rec.Version, rec.Cookbook, filesToCapture)
		if err != nil {
			return nil, err
		}
		allBacked = append(allBacked, backed...)
	}
	return allBacked, nil
}

func captureFilesToDir(destDir string, meta BackupMeta, paths []string) ([]string, error) {
	if err := os.MkdirAll(destDir, 0700); err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	var capturedRel []string
	var capturedAbs []string
	for _, src := range paths {
		if _, err := os.Stat(src); err != nil {
			continue // file doesn't exist yet; skip
		}
		rel, err := filepath.Rel(home, src)
		if err != nil {
			rel = filepath.Base(src)
		}
		dst := filepath.Join(destDir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
			return nil, err
		}
		if err := copyFile(src, dst); err != nil {
			return nil, err
		}
		capturedRel = append(capturedRel, rel)
		capturedAbs = append(capturedAbs, src)
	}

	meta.CapturedAt = time.Now()
	meta.Files = capturedRel
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, err
	}
	return capturedAbs, os.WriteFile(filepath.Join(destDir, "meta.json"), data, 0600)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// ListBackupSlots returns all backup slots found under ~/.kimchi/backups/.
func ListBackupSlots() ([]BackupSlot, error) {
	root, err := backupsRoot()
	if err != nil {
		return nil, err
	}
	toolDirs, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var slots []BackupSlot
	for _, td := range toolDirs {
		if !td.IsDir() {
			continue
		}
		toolID := tools.ToolID(td.Name())
		slotDirs, err := os.ReadDir(filepath.Join(root, td.Name()))
		if err != nil {
			continue
		}
		for _, sd := range slotDirs {
			if !sd.IsDir() {
				continue
			}
			dir := filepath.Join(root, td.Name(), sd.Name())
			metaData, err := os.ReadFile(filepath.Join(dir, "meta.json"))
			if err != nil {
				continue
			}
			var meta BackupMeta
			if err := json.Unmarshal(metaData, &meta); err != nil {
				continue
			}
			slots = append(slots, BackupSlot{
				Tool:       toolID,
				RecipeName: meta.RecipeName,
				CapturedAt: meta.CapturedAt,
				Dir:        dir,
				Meta:       meta,
			})
		}
	}
	return slots, nil
}

// RestoreSlot restores all files from a BackupSlot to their original locations
// under $HOME. It first removes all currently installed recipe asset files so
// that files present in the current install but absent from the backup slot are
// not left behind. After restore it updates kimchi state to match the slot.
func RestoreSlot(slot BackupSlot) error {
	// Remove all currently installed asset files before restoring, so the
	// on-disk state is fully replaced rather than merged.
	installed, err := loadInstalledForTool(slot.Tool)
	if err != nil {
		return fmt.Errorf("load installed recipes: %w", err)
	}
	for _, rec := range installed {
		if err := UninstallByManifest(slot.Tool, rec.Name); err != nil {
			return fmt.Errorf("uninstall %s before restore: %w", rec.Name, err)
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	for _, rel := range slot.Meta.Files {
		src := filepath.Join(slot.Dir, rel)
		dst := filepath.Join(home, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return fmt.Errorf("restore mkdir %s: %w", rel, err)
		}
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("restore %s: %w", rel, err)
		}
	}
	if slot.RecipeName != "" {
		// Per-recipe restore: this slot represents a known installed state,
		// so replace the installed list with just this recipe.
		_ = RecordInstall(slot.RecipeName, slot.Meta.RecipeVersion, slot.Meta.RecipeCookbook, slot.Tool)
	} else {
		// Baseline restore: wipe all install records and manifests for this tool.
		_ = clearAllInstalledForTool(slot.Tool)
		_ = clearAllManifestsForTool(slot.Tool)
	}
	return nil
}
