package steps

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/tools"
)

type installState int

const (
	installStateCheck installState = iota
	installStateOffer
	installStateInstalling
	installStateInstalled
	installStateSkipped
)

type InstallStep struct {
	state        installState
	hasTools     bool
	installError string
	selectedTool tools.ToolID
}

type installCheckCompleteMsg struct {
	hasTools bool
}

type installCompleteMsg struct {
	err error
}

func NewInstallStep() *InstallStep {
	return &InstallStep{
		state: installStateCheck,
	}
}

func (s *InstallStep) SelectedTool() tools.ToolID {
	return s.selectedTool
}

func (s *InstallStep) HasInstalledTools() bool {
	return s.hasTools && s.state == installStateCheck
}

func (s *InstallStep) AutoSelectedTools() []tools.ToolID {
	var selected []tools.ToolID
	for _, tool := range tools.All() {
		if tool.ID != tools.ToolGeneric && tool.DetectInstalled() {
			selected = append(selected, tool.ID)
		}
	}
	return selected
}

func (s *InstallStep) ShouldSkipToolsStep() bool {
	return s.state == installStateInstalled
}

func (s *InstallStep) Init() tea.Cmd {
	return s.checkForTools()
}

func (s *InstallStep) checkForTools() tea.Cmd {
	return func() tea.Msg {
		for _, tool := range tools.All() {
			if tool.ID != tools.ToolGeneric && tool.DetectInstalled() {
				return installCheckCompleteMsg{hasTools: true}
			}
		}
		return installCheckCompleteMsg{hasTools: false}
	}
}

func (s *InstallStep) installOpenCode() tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("bash", "-c", "curl -fsSL https://raw.githubusercontent.com/opencode-ai/opencode/main/install.sh | bash")
		case "linux":
			cmd = exec.Command("bash", "-c", "curl -fsSL https://raw.githubusercontent.com/opencode-ai/opencode/main/install.sh | bash")
		default:
			return installCompleteMsg{err: fmt.Errorf("unsupported platform: %s", runtime.GOOS)}
		}

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		err := cmd.Run()
		return installCompleteMsg{err: err}
	}
}

func (s *InstallStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return s, func() tea.Msg { return AbortMsg{} }
		case "esc":
			return s, func() tea.Msg { return PrevStepMsg{} }
		case "y", "Y":
			if s.state == installStateOffer {
				s.state = installStateInstalling
				return s, s.installOpenCode()
			}
		case "n", "N":
			if s.state == installStateOffer {
				s.state = installStateSkipped
				return s, func() tea.Msg { return NextStepMsg{} }
			}
		}

	case installCheckCompleteMsg:
		s.hasTools = msg.hasTools
		if s.hasTools {
			return s, func() tea.Msg { return RemoveStepMsg{} }
		}
		s.state = installStateOffer
		return s, nil

	case installCompleteMsg:
		if msg.err != nil {
			s.state = installStateSkipped
			s.installError = msg.err.Error()
			return s, func() tea.Msg { return NextStepMsg{} }
		}
		s.state = installStateInstalled
		s.selectedTool = tools.ToolOpenCode
		return s, func() tea.Msg { return SkipToolsAndRemoveMsg{} }
	}

	return s, nil
}

func (s *InstallStep) View() string {
	var b strings.Builder

	switch s.state {
	case installStateCheck:
		b.WriteString(Styles.Spinner.Render("Checking for installed tools..."))

	case installStateOffer:
		b.WriteString(Styles.Title.Render("No AI coding tools detected"))
		b.WriteString("\n\n")
		b.WriteString("We couldn't find any supported AI coding tools.\n\n")
		b.WriteString("OpenCode is a powerful agentic coding CLI that works\n")
		b.WriteString("great with Cast AI's open-source models.\n\n")
		b.WriteString("Install OpenCode now?\n")

	case installStateInstalling:
		b.WriteString(Styles.Title.Render("Installing OpenCode"))
		b.WriteString("\n\n")
		b.WriteString("Running installer...")
		b.WriteString("\n")
		b.WriteString(Styles.Help.Render("Please wait, this may take a moment."))

	case installStateInstalled:
		b.WriteString(Styles.Title.Render("OpenCode Installed"))
		b.WriteString("\n\n")
		b.WriteString(Styles.Success.Render("✓ Installation successful!"))
		b.WriteString("\n\n")
		b.WriteString("We'll configure it for you automatically.")

	case installStateSkipped:
		if s.installError != "" {
			b.WriteString(Styles.Title.Render("Installation Issue"))
			b.WriteString("\n\n")
			b.WriteString(Styles.Warning.Render("⚠ Could not install OpenCode"))
			b.WriteString("\n\n")
			b.WriteString(Styles.Desc.Render(s.installError))
			b.WriteString("\n\n")
			b.WriteString("You can select tools manually in the next step.")
		}
	}

	return b.String()
}

func (s *InstallStep) Name() string {
	return "Check Tools"
}

func (s *InstallStep) Info() StepInfo {
	bindings := []KeyBinding{BindingsQuit}
	if s.state == installStateOffer {
		bindings = append(bindings, KeyBinding{Key: "y", Text: "install"}, KeyBinding{Key: "n", Text: "skip"})
	}
	return StepInfo{
		Name:        "Check Tools",
		KeyBindings: bindings,
	}
}
