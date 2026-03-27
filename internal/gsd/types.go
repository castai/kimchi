package gsd

type InstallationType string

const (
	InstallationOpenCode   InstallationType = "opencode"
	InstallationClaudeCode InstallationType = "claude-code"
	InstallationCodex      InstallationType = "codex"
)

var PlanningAgents = []string{
	"gsd-planner",
	"gsd-plan-checker",
	"gsd-phase-researcher",
	"gsd-project-researcher",
	"gsd-roadmapper",
	"gsd-codebase-mapper",
	"gsd-research-synthesizer",
}

var ExecutionAgents = []string{
	"gsd-executor",
	"gsd-debugger",
	"gsd-verifier",
	"gsd-integration-checker",
	"gsd-nyquist-auditor",
}

type AgentFile struct {
	Path       string
	Name       string
	RawContent string
}

type Installation struct {
	Type       InstallationType
	Path       string
	AgentFiles []AgentFile
}
