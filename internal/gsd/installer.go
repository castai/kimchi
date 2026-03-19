package gsd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Installer struct{}

func NewInstaller() *Installer {
	return &Installer{}
}

type InstallResult struct {
	Type      InstallationType
	Path      string
	Installed []string
}

func (i *Installer) Install(installType InstallationType, scope string) (*InstallResult, error) {
	switch installType {
	case InstallationOpenCode:
		return i.installOpenCode(scope)
	case InstallationClaudeCode:
		return i.installClaudeCode(scope)
	case InstallationCodex:
		return i.installCodex(scope)
	default:
		return nil, fmt.Errorf("unsupported tool for GSD: %s", installType)
	}
}

func (i *Installer) installOpenCode(scope string) (*InstallResult, error) {
	args := []string{"--yes", "gsd-opencode@latest"}
	if scope == "project" {
		args = append(args, "--local")
	} else {
		args = append(args, "--global")
	}

	cmd := exec.Command("npx", args...)
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("run gsd-opencode installer: %w", err)
	}

	basePath, err := getOpenCodeGSDPath(scope)
	if err != nil {
		basePath = "installed"
	}

	return &InstallResult{
		Type:      InstallationOpenCode,
		Path:      basePath,
		Installed: []string{"gsd-opencode"},
	}, nil
}

func (i *Installer) installClaudeCode(scope string) (*InstallResult, error) {
	args := []string{"--yes", "get-shit-done-cc@latest", "--claude"}
	if scope == "project" {
		args = append(args, "--local")
	} else {
		args = append(args, "--global")
	}

	cmd := exec.Command("npx", args...)
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("run get-shit-done-cc installer: %w", err)
	}

	basePath, err := getClaudeCodeGSDPath(scope)
	if err != nil {
		basePath = "installed"
	}

	return &InstallResult{
		Type:      InstallationClaudeCode,
		Path:      basePath,
		Installed: []string{"get-shit-done-cc"},
	}, nil
}

func getOpenCodeGSDPath(scope string) (string, error) {
	if scope == "project" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(cwd, ".opencode", "commands", "gsd"), nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "opencode", "commands", "gsd"), nil
}

func getClaudeCodeGSDPath(scope string) (string, error) {
	if scope == "project" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(cwd, ".claude", "commands", "gsd"), nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".claude", "commands", "gsd"), nil
}

func (i *Installer) installCodex(scope string) (*InstallResult, error) {
	args := []string{"--yes", "get-shit-done-cc@latest", "--codex"}
	if scope == "project" {
		args = append(args, "--local")
	} else {
		args = append(args, "--global")
	}

	cmd := exec.Command("npx", args...)
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("run get-shit-done-cc installer: %w", err)
	}

	basePath, err := getCodexGSDPath(scope)
	if err != nil {
		basePath = "installed"
	}

	return &InstallResult{
		Type:      InstallationCodex,
		Path:      basePath,
		Installed: []string{"get-shit-done-cc"},
	}, nil
}

func getCodexGSDPath(scope string) (string, error) {
	if scope == "project" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(cwd, ".codex", "commands", "gsd"), nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".codex", "commands", "gsd"), nil
}

func (i *Installer) IsInstalledFor(installType InstallationType, scope string) bool {
	var basePath string
	var err error

	switch installType {
	case InstallationOpenCode:
		basePath, err = getOpenCodeGSDPath(scope)
	case InstallationClaudeCode:
		basePath, err = getClaudeCodeGSDPath(scope)
	case InstallationCodex:
		basePath, err = getCodexGSDPath(scope)
	default:
		return false
	}

	if err != nil {
		return false
	}

	info, err := os.Stat(basePath)
	if err != nil {
		return false
	}

	return info.IsDir()
}
