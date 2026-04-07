package recipe

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/castai/kimchi/internal/tools"
)

// InstalledRecipe tracks a recipe that has been installed on this machine.
type InstalledRecipe struct {
	Name        string       `json:"name"`
	Version     string       `json:"version"`
	Cookbook    string       `json:"cookbook,omitempty"`
	Tool        tools.ToolID `json:"tool,omitempty"`
	InstalledAt time.Time    `json:"installed_at"`
	Pinned      bool         `json:"pinned"`
}

// LoadInstalled returns all installed recipes across all tools.
func LoadInstalled() ([]InstalledRecipe, error) {
	if err := migrateInstalledIfNeeded(); err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".kimchi", "installed")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var all []InstalledRecipe
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		toolID := tools.ToolID(strings.TrimSuffix(e.Name(), ".json"))
		recs, err := loadInstalledForTool(toolID)
		if err != nil {
			continue
		}
		all = append(all, recs...)
	}
	return all, nil
}

// RecordInstall saves the install record for a recipe, replacing any previously
// installed recipe for the same tool (only one recipe per tool at a time).
func RecordInstall(name, version, cookbook string, tool tools.ToolID) error {
	return saveInstalledForTool(tool, []InstalledRecipe{
		{
			Name:        name,
			Version:     version,
			Cookbook:    cookbook,
			Tool:        tool,
			InstalledAt: time.Now(),
		},
	})
}

// GetLastInstalled returns the most recently installed recipe across all tools, or nil if none.
func GetLastInstalled() (*InstalledRecipe, error) {
	list, err := LoadInstalled()
	if err != nil {
		return nil, err
	}
	var latest *InstalledRecipe
	for i := range list {
		if latest == nil || list[i].InstalledAt.After(latest.InstalledAt) {
			latest = &list[i]
		}
	}
	return latest, nil
}

// GetInstalled returns the install record for a recipe by name, or nil if not installed.
func GetInstalled(name string) (*InstalledRecipe, error) {
	list, err := LoadInstalled()
	if err != nil {
		return nil, err
	}
	for i, r := range list {
		if r.Name == name {
			return &list[i], nil
		}
	}
	return nil, nil
}

// Pin marks a recipe as pinned (will not be upgraded automatically).
func Pin(name string) error {
	return setPinned(name, true)
}

// Unpin removes the pin from a recipe.
func Unpin(name string) error {
	return setPinned(name, false)
}

func setPinned(name string, pinned bool) error {
	list, err := LoadInstalled()
	if err != nil {
		return err
	}
	for _, r := range list {
		if r.Name == name {
			toolList, err := loadInstalledForTool(r.Tool)
			if err != nil {
				return err
			}
			for i, tr := range toolList {
				if tr.Name == name {
					toolList[i].Pinned = pinned
					return saveInstalledForTool(r.Tool, toolList)
				}
			}
		}
	}
	return fmt.Errorf("recipe %q is not installed", name)
}
