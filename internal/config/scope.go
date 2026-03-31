package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type ConfigScope string

const (
	ScopeGlobal  ConfigScope = "global"
	ScopeProject ConfigScope = "project"
)

func ScopePaths(scope ConfigScope, toolConfigPath string) (string, error) {
	switch scope {
	case ScopeGlobal:
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home directory: %w", err)
		}
		if len(toolConfigPath) > 1 && toolConfigPath[0] == '~' {
			return filepath.Join(homeDir, toolConfigPath[1:]), nil
		}
		return toolConfigPath, nil

	case ScopeProject:
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
		return filepath.Join(cwd, ".claude", filepath.Base(toolConfigPath)), nil

	default:
		return "", fmt.Errorf("unknown scope: %s", scope)
	}
}

func ParseScope(s string) (ConfigScope, error) {
	switch s {
	case "global", "":
		return ScopeGlobal, nil
	case "project":
		return ScopeProject, nil
	default:
		return "", fmt.Errorf("invalid scope: %s (use 'global' or 'project')", s)
	}
}
