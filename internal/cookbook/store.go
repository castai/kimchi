package cookbook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func storePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kimchi", "cookbooks.json"), nil
}

// DefaultCloneDir returns the default directory for cloning cookbooks.
func DefaultCloneDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kimchi", "cookbooks"), nil
}

// Load returns all cookbooks: the default cookbook (if enabled and cloned)
// followed by all user-registered cookbooks.
func Load() ([]Cookbook, error) {
	p, err := storePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		data = nil
	} else if err != nil {
		return nil, fmt.Errorf("read cookbooks: %w", err)
	}

	var userList []Cookbook
	if len(data) > 0 {
		if err := json.Unmarshal(data, &userList); err != nil {
			return nil, fmt.Errorf("parse cookbooks: %w", err)
		}
	}

	// Prepend the default cookbook if it is enabled and already cloned on disk.
	// We do not clone here — EnsureDefault (called by AutoUpdateIfStale) handles that.
	defURL := DefaultCookbookURL()
	if defURL != "" {
		defPath, pathErr := defaultCookbookPath()
		if pathErr == nil {
			if _, statErr := os.Stat(filepath.Join(defPath, ".git")); statErr == nil {
				// Only prepend if the user hasn't registered a cookbook with the same name.
				alreadyRegistered := false
				for _, c := range userList {
					if c.Name == DefaultCookbookName {
						alreadyRegistered = true
						break
					}
				}
				if !alreadyRegistered {
					def := Cookbook{Name: DefaultCookbookName, URL: defURL, Path: defPath}
					userList = append([]Cookbook{def}, userList...)
				}
			}
		}
	}

	return userList, nil
}

// loadUserList reads only the user-registered cookbooks from disk (the JSON
// store). It does NOT include the built-in default cookbook. Use this for
// mutations (Add, Remove) so the default is never accidentally persisted.
func loadUserList() ([]Cookbook, error) {
	p, err := storePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read cookbooks: %w", err)
	}
	var list []Cookbook
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parse cookbooks: %w", err)
	}
	return list, nil
}

func save(list []Cookbook) error {
	p, err := storePath()
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

// Add registers a new cookbook. Returns an error if a cookbook with the same name is already registered.
func Add(cb Cookbook) error {
	list, err := loadUserList()
	if err != nil {
		return err
	}
	for _, c := range list {
		if c.Name == cb.Name {
			return fmt.Errorf("cookbook %q is already registered (path: %s)", cb.Name, c.Path)
		}
	}
	return save(append(list, cb))
}

// Remove unregisters a cookbook by name.
// The default cookbook cannot be removed via this function; set
// KIMCHI_DEFAULT_COOKBOOK_URL="" to disable it instead.
func Remove(name string) error {
	if IsDefault(name) && DefaultCookbookURL() != "" {
		return fmt.Errorf("cannot remove the default cookbook %q; set KIMCHI_DEFAULT_COOKBOOK_URL= to disable it", name)
	}
	list, err := loadUserList()
	if err != nil {
		return err
	}
	next := list[:0]
	found := false
	for _, c := range list {
		if c.Name == name {
			found = true
			continue
		}
		next = append(next, c)
	}
	if !found {
		return fmt.Errorf("cookbook %q not found", name)
	}
	return save(next)
}

// Get returns a cookbook by name, or nil if not found.
func Get(name string) (*Cookbook, error) {
	list, err := Load()
	if err != nil {
		return nil, err
	}
	for i, c := range list {
		if c.Name == name {
			return &list[i], nil
		}
	}
	return nil, nil
}

// NameFromURL derives a cookbook name from its remote URL.
// e.g. "https://github.com/alice/my-cookbook.git" → "my-cookbook"
func NameFromURL(rawURL string) string {
	// Strip trailing .git
	name := strings.TrimSuffix(rawURL, ".git")
	// Take the last path component
	if idx := strings.LastIndexAny(name, "/:\\"); idx >= 0 {
		name = name[idx+1:]
	}
	return name
}
