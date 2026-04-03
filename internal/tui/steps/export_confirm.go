package steps

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type exportConfirmState int

const (
	exportConfirmIdle exportConfirmState = iota
	exportConfirmWriting
	exportConfirmDone
	exportConfirmError
)

type exportWriteCompleteMsg struct {
	err error
}

// ExportConfirmStep shows a summary of what will be exported, performs the
// async write via writeFn, and shows the result.
type ExportConfirmStep struct {
	outputPath string
	writeFn    func() error
	state      exportConfirmState
	err        error
	spin       spinner.Model

	// summary fields for display
	name        string
	author      string
	useCase     string
	included    []string
}

// NewExportConfirmStep creates the final export step.
// writeFn is called when the user confirms; it should read assets, build and
// write the recipe. This avoids importing the recipe package from the steps package.
func NewExportConfirmStep(outputPath string, writeFn func() error, name, author, useCase string, included []string) *ExportConfirmStep {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return &ExportConfirmStep{
		outputPath: outputPath,
		writeFn:    writeFn,
		state:      exportConfirmIdle,
		spin:       sp,
		name:       name,
		author:     author,
		useCase:    useCase,
		included:   included,
	}
}

func (s *ExportConfirmStep) Init() tea.Cmd { return nil }

func (s *ExportConfirmStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return s, func() tea.Msg { return AbortMsg{} }
		case "esc":
			if s.state == exportConfirmIdle {
				return s, func() tea.Msg { return PrevStepMsg{} }
			}
		case "enter":
			switch s.state {
			case exportConfirmIdle:
				s.state = exportConfirmWriting
				return s, tea.Batch(s.spin.Tick, s.doWrite())
			case exportConfirmDone, exportConfirmError:
				return s, func() tea.Msg { return NextStepMsg{} }
			}
		}

	case exportWriteCompleteMsg:
		if msg.err != nil {
			s.state = exportConfirmError
			s.err = msg.err
		} else {
			s.state = exportConfirmDone
		}
		return s, nil

	case spinner.TickMsg:
		if s.state == exportConfirmWriting {
			var cmd tea.Cmd
			s.spin, cmd = s.spin.Update(msg)
			return s, cmd
		}
	}

	return s, nil
}

func (s *ExportConfirmStep) doWrite() tea.Cmd {
	return func() tea.Msg {
		return exportWriteCompleteMsg{err: s.writeFn()}
	}
}

func (s *ExportConfirmStep) View() string {
	var b strings.Builder

	switch s.state {
	case exportConfirmIdle:
		b.WriteString("Ready to export the following recipe:\n\n")
		b.WriteString(fmt.Sprintf("  %s %s\n", Styles.Desc.Render("Name:"), s.name))
		b.WriteString(fmt.Sprintf("  %s %s\n", Styles.Desc.Render("Author:"), s.author))
		b.WriteString(fmt.Sprintf("  %s %s\n", Styles.Desc.Render("Use case:"), s.useCase))
		if len(s.included) > 0 {
			b.WriteString(fmt.Sprintf("  %s %s\n", Styles.Desc.Render("Assets:"), strings.Join(s.included, ", ")))
		}
		b.WriteString(fmt.Sprintf("\n  %s %s\n", Styles.Desc.Render("Output:"), s.outputPath))
		b.WriteString("\n")
		b.WriteString(Styles.Help.Render("Press enter to export, esc to go back"))

	case exportConfirmWriting:
		b.WriteString(Styles.Spinner.Render(fmt.Sprintf("%s Writing recipe...", s.spin.View())))

	case exportConfirmDone:
		b.WriteString(Styles.Success.Render("✓ Recipe exported successfully"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("  %s\n", s.outputPath))
		b.WriteString("\n")
		b.WriteString(Styles.Help.Render("Press enter to exit"))

	case exportConfirmError:
		b.WriteString(Styles.Error.Render(fmt.Sprintf("✗ Export failed: %v", s.err)))
		b.WriteString("\n\n")
		b.WriteString(Styles.Help.Render("Press enter to exit"))
	}

	return b.String()
}

// SetOutputPath updates the output path shown in the summary and used in the done message.
func (s *ExportConfirmStep) SetOutputPath(path string) {
	s.outputPath = path
}

// SetSummary updates the display fields shown in the idle state summary.
// Called by the wizard after collecting all prior step results.
func (s *ExportConfirmStep) SetSummary(name, author, useCase string, included []string) {
	s.name = name
	s.author = author
	s.useCase = useCase
	s.included = included
}

func (s *ExportConfirmStep) Name() string { return "Export" }

func (s *ExportConfirmStep) Info() StepInfo {
	bindings := []KeyBinding{BindingsBack, BindingsQuit}
	switch s.state {
	case exportConfirmIdle:
		bindings = []KeyBinding{BindingsConfirm, BindingsBack, BindingsQuit}
	case exportConfirmDone, exportConfirmError:
		bindings = []KeyBinding{BindingsConfirm}
	}
	return StepInfo{
		Name:        "Export",
		KeyBindings: bindings,
	}
}
