package gsd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Detector struct{}

func NewDetector() *Detector {
	return &Detector{}
}

// Detect scans the global install locations for all supported tools and returns
// every Installation that has GSD agent files present.
func (d *Detector) Detect() ([]Installation, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	opencodeRoot, err := resolveOpenCodeGlobalRoot()
	if err != nil {
		opencodeRoot = filepath.Join(homeDir, ".config", "opencode")
	}

	var installations []Installation
	if install := d.detectInRoot(opencodeRoot, InstallationOpenCode); install != nil {
		installations = append(installations, *install)
	}

	return installations, nil
}

// detectInRoot looks for GSD agent files under root, checking both the current
// (commands/gsd) and legacy (command/gsd) directory structures. If neither
// directory has agent files but the gsd-opencode VERSION file exists, a bare
// Installation is returned so the caller knows something is installed even
// before any agent files are written.
func (d *Detector) detectInRoot(root string, installType InstallationType) *Installation {
	for _, dir := range gsdCommandDirs(root) {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		files, err := d.agentFiles(dir)
		if err != nil || len(files) == 0 {
			continue
		}
		return &Installation{Type: installType, Path: dir, AgentFiles: files}
	}

	// VERSION file present but command dir empty or absent (e.g. partial install).
	if installType == InstallationOpenCode {
		versionFile := filepath.Join(root, "get-shit-done", "VERSION")
		if _, err := os.Stat(versionFile); err == nil {
			return &Installation{Type: installType, Path: root}
		}
	}

	return nil
}

func (d *Detector) agentFiles(dir string) ([]AgentFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	var files []AgentFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		fullPath := filepath.Join(dir, entry.Name())
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		files = append(files, AgentFile{
			Path:       fullPath,
			Name:       strings.TrimSuffix(entry.Name(), ".md"),
			RawContent: string(content),
		})
	}
	return files, nil
}
