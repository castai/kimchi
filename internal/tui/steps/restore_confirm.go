package steps

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/recipe"
)

type restoreCompleteMsg struct{ err error }

// RestoreConfirmStep shows the selected slot details, asks for confirmation,
// then performs the restore.
type RestoreConfirmStep struct {
	slot *recipe.BackupSlot
	done bool
	err  error
}

func NewRestoreConfirmStep(slot *recipe.BackupSlot) *RestoreConfirmStep {
	return &RestoreConfirmStep{slot: slot}
}

func (s *RestoreConfirmStep) Init() tea.Cmd { return nil }

func (s *RestoreConfirmStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case restoreCompleteMsg:
		s.done = true
		s.err = msg.err
		return s, nil

	case tea.KeyMsg:
		if s.done {
			switch msg.String() {
			case "enter", "q", "ctrl+c":
				return s, func() tea.Msg { return NextStepMsg{} }
			}
			return s, nil
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return s, func() tea.Msg { return AbortMsg{} }
		case "esc", "n":
			return s, func() tea.Msg { return PrevStepMsg{} }
		case "y", "enter":
			return s, s.doRestore()
		}
	}
	return s, nil
}

func (s *RestoreConfirmStep) doRestore() tea.Cmd {
	slot := *s.slot
	return func() tea.Msg {
		return restoreCompleteMsg{err: recipe.RestoreSlot(slot)}
	}
}

func (s *RestoreConfirmStep) View() string {
	var b strings.Builder
	if s.slot == nil {
		return "No backup selected.\n"
	}

	if !s.done {
		name := s.slot.RecipeName
		if name == "" {
			name = "baseline (pre-install state)"
		}
		b.WriteString(fmt.Sprintf("Tool:      %s\n", s.slot.Tool))
		b.WriteString(fmt.Sprintf("Backup:    %s\n", name))
		b.WriteString(fmt.Sprintf("Captured:  %s\n\n", s.slot.CapturedAt.Format("2006-01-02 15:04:05")))

		b.WriteString(fmt.Sprintf("Files to restore (%d):\n", len(s.slot.Meta.Files)))
		for _, f := range s.slot.Meta.Files {
			b.WriteString("  " + Styles.Desc.Render(f) + "\n")
		}
		b.WriteString("\n")
		b.WriteString(Styles.Warning.Render("This will overwrite your current config. Continue?"))
		b.WriteString("  ")
		b.WriteString(Styles.Key.Render("y") + Styles.Help.Render("/↵ yes") + "  ")
		b.WriteString(Styles.Key.Render("n") + Styles.Help.Render("/esc no"))
		return b.String()
	}

	if s.err != nil {
		b.WriteString(Styles.Error.Render(fmt.Sprintf("✗ Restore failed: %v", s.err)))
	} else {
		b.WriteString(Styles.Success.Render("✓ Restore complete!"))
		if s.slot.RecipeName != "" {
			b.WriteString("\n")
			b.WriteString(Styles.Help.Render(fmt.Sprintf(
				"Recipe %q removed from installed list. Use `kimchi recipe install` to re-install.",
				s.slot.RecipeName,
			)))
		}
	}
	b.WriteString("\n\n")
	b.WriteString(Styles.Help.Render("Press enter to exit"))
	return b.String()
}

func (s *RestoreConfirmStep) Name() string { return "Confirm Restore" }

func (s *RestoreConfirmStep) Info() StepInfo {
	if s.done {
		return StepInfo{Name: "Confirm Restore", KeyBindings: []KeyBinding{BindingsConfirm}}
	}
	return StepInfo{
		Name: "Confirm Restore",
		KeyBindings: []KeyBinding{
			{Key: "y/↵", Text: "confirm"},
			{Key: "n/esc", Text: "cancel"},
			BindingsQuit,
		},
	}
}
