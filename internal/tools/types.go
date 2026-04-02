package tools

import "github.com/castai/kimchi/internal/config"

type ToolID string

const (
	ToolOpenCode ToolID = "opencode"
	ToolContinue ToolID = "continue"
	ToolWindsurf ToolID = "windsurf"
	ToolZed      ToolID = "zed"
	ToolCodex    ToolID = "codex"
	ToolCline    ToolID = "cline"
	ToolGSD2     ToolID = "gsd2"
	ToolGeneric  ToolID = "generic"
)

type Tool struct {
	ID          ToolID
	Name        string
	Description string
	ConfigPath  string
	BinaryName  string
	IsInstalled func() bool
	Write       func(scope config.ConfigScope) error
}

func (t Tool) DetectInstalled() bool {
	if t.IsInstalled == nil {
		return false
	}
	return t.IsInstalled()
}
