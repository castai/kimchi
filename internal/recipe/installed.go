package recipe

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

func installedPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kimchi", "installed.json"), nil
}

// LoadInstalled returns all installed recipes.
func LoadInstalled() ([]InstalledRecipe, error) {
	p, err := installedPath()
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

func saveInstalled(list []InstalledRecipe) error {
	p, err := installedPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

// RecordInstall adds or updates the install record for a recipe.
func RecordInstall(name, version, cookbook string, tool tools.ToolID) error {
	list, err := LoadInstalled()
	if err != nil {
		return err
	}
	for i, r := range list {
		if r.Name == name && r.Tool == tool {
			list[i].Version = version
			list[i].Cookbook = cookbook
			list[i].InstalledAt = time.Now()
			return saveInstalled(list)
		}
	}
	list = append(list, InstalledRecipe{
		Name:        name,
		Version:     version,
		Cookbook:    cookbook,
		Tool:        tool,
		InstalledAt: time.Now(),
	})
	return saveInstalled(list)
}

// GetLastInstalled returns the most recently installed recipe, or nil if none.
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

// GetInstalled returns the install record for a recipe, or nil if not installed.
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
	for i, r := range list {
		if r.Name == name {
			list[i].Pinned = pinned
			return saveInstalled(list)
		}
	}
	return fmt.Errorf("recipe %q is not installed", name)
}
