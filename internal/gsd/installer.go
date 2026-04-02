package gsd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
		return i.installGetShitDone(scope, "--claude", InstallationClaudeCode)
	case InstallationCodex:
		return i.installGetShitDone(scope, "--codex", InstallationCodex)
	default:
		return nil, fmt.Errorf("unsupported tool for GSD: %s", installType)
	}
}

func (i *Installer) installOpenCode(scope string) (*InstallResult, error) {
	scopeFlag := "--global"
	if scope == "project" {
		scopeFlag = "--local"
	}

	if i.IsInstalledFor(InstallationOpenCode, scope) {
		// Already installed: run the repair subcommand non-interactively.
		// The repair prompt is a confirm (y/N) that accepts plain text on stdin.
		cmd := exec.Command("npx", "--yes", "gsd-opencode@latest", "repair", scopeFlag)
		cmd.Stdin = strings.NewReader("y\n")
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("run gsd-opencode repair: %w", err)
		}
	} else {
		// Fresh install: no stdin keeps the process non-interactive.
		cmd := exec.Command("npx", "--yes", "gsd-opencode@latest", scopeFlag)
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("run gsd-opencode installer: %w", err)
		}
	}

	root, _ := configRoot(InstallationOpenCode, scope)
	return &InstallResult{
		Type:      InstallationOpenCode,
		Path:      filepath.Join(root, "commands", "gsd"),
		Installed: []string{"gsd-opencode"},
	}, nil
}

// installGetShitDone handles Claude Code and Codex, which both use the
// get-shit-done-cc package and never prompt when runtime+scope flags are given.
func (i *Installer) installGetShitDone(scope, runtimeFlag string, installType InstallationType) (*InstallResult, error) {
	scopeFlag := "--global"
	if scope == "project" {
		scopeFlag = "--local"
	}

	// No stdin: the statusline and SDK prompts both guard on isTTY and skip
	// when stdin is not a terminal.
	cmd := exec.Command("npx", "--yes", "get-shit-done-cc@latest", runtimeFlag, scopeFlag)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("run get-shit-done-cc installer: %w", err)
	}

	root, _ := configRoot(installType, scope)
	return &InstallResult{
		Type:      installType,
		Path:      filepath.Join(root, "commands", "gsd"),
		Installed: []string{"get-shit-done-cc"},
	}, nil
}

// IsInstalledFor reports whether GSD is already installed for the given tool
// and scope, using the same signals the npm packages use internally so that
// our routing (repair vs fresh install) agrees with theirs.
func (i *Installer) IsInstalledFor(installType InstallationType, scope string) bool {
	root, err := configRoot(installType, scope)
	if err != nil {
		return false
	}

	if installType == InstallationOpenCode {
		// Primary signal used by gsd-opencode: VERSION file.
		versionFile := filepath.Join(root, "get-shit-done", "VERSION")
		if _, err := os.Stat(versionFile); err == nil {
			return true
		}
	}

	// Current (commands/gsd) and legacy (command/gsd) directory structures.
	for _, dir := range gsdCommandDirs(root) {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}
