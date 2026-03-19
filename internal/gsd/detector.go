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

func (d *Detector) Detect() ([]Installation, error) {
	var installations []Installation

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	openCodeGSD := filepath.Join(homeDir, ".config", "opencode", "commands", "gsd")
	if install, err := d.checkInstallation(openCodeGSD, InstallationOpenCode); err == nil && install != nil {
		installations = append(installations, *install)
	}

	claudeCodeGSD := filepath.Join(homeDir, ".claude", "commands", "gsd")
	if install, err := d.checkInstallation(claudeCodeGSD, InstallationClaudeCode); err == nil && install != nil {
		installations = append(installations, *install)
	}

	return installations, nil
}

func (d *Detector) checkInstallation(path string, installType InstallationType) (*Installation, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}

	if !info.IsDir() {
		return nil, nil
	}

	agentFiles, err := d.getAgentFiles(path)
	if err != nil {
		return nil, fmt.Errorf("get agent files: %w", err)
	}

	if len(agentFiles) == 0 {
		return nil, nil
	}

	return &Installation{
		Type:       installType,
		Path:       path,
		AgentFiles: agentFiles,
	}, nil
}

func (d *Detector) getAgentFiles(dir string) ([]AgentFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	var files []AgentFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}

		fullPath := filepath.Join(dir, name)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		agentName := strings.TrimSuffix(name, ".md")
		files = append(files, AgentFile{
			Path:       fullPath,
			Name:       agentName,
			RawContent: string(content),
		})
	}

	return files, nil
}
