package steps

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/tools"
)

const openCodeInstallURL = "https://opencode.ai/install"

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

func downloadFile(url, destPath string) error {
	resp, err := http.Get(url) //nolint:gosec,noctx // URL is a compile-time constant.
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download %s: HTTP %d %s", url, resp.StatusCode, resp.Status)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", destPath, err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return fmt.Errorf("failed to write file %s: %w", destPath, err)
	}
	return f.Close()
}

func (s *InstallStep) installOpenCode() tea.Cmd {
	return func() tea.Msg {
		switch runtime.GOOS {
		case "darwin", "linux":
		default:
			return installCompleteMsg{err: fmt.Errorf("unsupported platform: %s", runtime.GOOS)}
		}

		tmpDir, err := os.MkdirTemp("", "kimchi-opencode-install")
		if err != nil {
			return installCompleteMsg{err: fmt.Errorf("failed to create temp directory: %w", err)}
		}
		defer os.RemoveAll(tmpDir)

		scriptPath := filepath.Join(tmpDir, "install.sh")
		if err := downloadFile(openCodeInstallURL, scriptPath); err != nil {
			return installCompleteMsg{err: err}
		}

		if err := os.Chmod(scriptPath, 0o700); err != nil {
			return installCompleteMsg{err: fmt.Errorf("failed to make install script executable: %w", err)}
		}

		var outputBuf bytes.Buffer
		cmd := exec.Command(scriptPath)
		cmd.Stdout = &outputBuf
		cmd.Stderr = &outputBuf
		if err := cmd.Run(); err != nil {
			return installCompleteMsg{
				err: fmt.Errorf("OpenCode install script failed: %s (%w)", strings.TrimSpace(outputBuf.String()), err),
			}
		}

		return installCompleteMsg{}
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
		case "enter":
			if s.state == installStateSkipped {
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
			return s, nil
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
			b.WriteString("For installation instructions visit: https://opencode.ai/docs\n\n")
			b.WriteString("You can select tools manually in the next step.\n\n")
			b.WriteString(Styles.Help.Render("Press enter to continue."))
		}
	}

	return b.String()
}

func (s *InstallStep) Name() string {
	return "Check Tools"
}

func (s *InstallStep) Info() StepInfo {
	bindings := []KeyBinding{BindingsQuit}
	switch s.state {
	case installStateOffer:
		bindings = append(bindings, KeyBinding{Key: "y", Text: "install"}, KeyBinding{Key: "n", Text: "skip"})
	case installStateSkipped:
		bindings = append(bindings, KeyBinding{Key: "↵", Text: "continue"})
	}
	return StepInfo{
		Name:        "Check Tools",
		KeyBindings: bindings,
	}
}
