package steps

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/castai/kimchi/internal/recipe"
)

type restoreLoadMsg struct {
	slots []recipe.BackupSlot
	err   error
}

const restorePageSize = 14

// RestorePickerStep loads backup slots and lets the user pick one.
type RestorePickerStep struct {
	slots    []recipe.BackupSlot
	cursor   int
	offset   int
	selected *recipe.BackupSlot
	loading  bool
	err      error
}

func NewRestorePickerStep() *RestorePickerStep {
	return &RestorePickerStep{loading: true}
}

func (s *RestorePickerStep) SelectedSlot() *recipe.BackupSlot { return s.selected }

func (s *RestorePickerStep) Init() tea.Cmd {
	return func() tea.Msg {
		slots, err := recipe.ListBackupSlots()
		return restoreLoadMsg{slots: slots, err: err}
	}
}

func (s *RestorePickerStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case restoreLoadMsg:
		s.loading = false
		s.err = msg.err
		s.slots = msg.slots
		return s, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return s, func() tea.Msg { return AbortMsg{} }
		case "esc":
			return s, func() tea.Msg { return PrevStepMsg{} }
		}
		if s.loading || len(s.slots) == 0 {
			return s, nil
		}
		switch msg.String() {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
				if s.cursor < s.offset {
					s.offset = s.cursor
				}
			}
		case "down", "j":
			if s.cursor < len(s.slots)-1 {
				s.cursor++
				if s.cursor >= s.offset+restorePageSize {
					s.offset = s.cursor - restorePageSize + 1
				}
			}
		case "enter":
			slot := s.slots[s.cursor]
			s.selected = &slot
			return s, func() tea.Msg { return NextStepMsg{} }
		}
	}
	return s, nil
}

func (s *RestorePickerStep) View() string {
	var b strings.Builder
	if s.loading {
		b.WriteString(Styles.Spinner.Render("Loading backups…") + "\n")
		return b.String()
	}
	if s.err != nil {
		b.WriteString(Styles.Error.Render(fmt.Sprintf("Error loading backups: %v", s.err)) + "\n")
		return b.String()
	}
	if len(s.slots) == 0 {
		b.WriteString("No backups found.\n\n")
		b.WriteString(Styles.Help.Render("Install a recipe first to create a backup.") + "\n")
		return b.String()
	}

	b.WriteString("Select a backup to restore.\n\n")

	total := len(s.slots)
	end := s.offset + restorePageSize
	if end > total {
		end = total
	}
	if s.offset > 0 {
		b.WriteString(Styles.Desc.Render(fmt.Sprintf("  ↑ %d more above\n", s.offset)))
	}
	for i := s.offset; i < end; i++ {
		slot := s.slots[i]
		cursor := "  "
		if s.cursor == i {
			cursor = Styles.Cursor.Render("► ")
		}
		toolLabel := string(slot.Tool) + " / "
		var nameLabel string
		if slot.RecipeName == "" {
			nameLabel = Styles.Warning.Render("baseline")
		} else {
			nameLabel = slot.RecipeName
		}
		date := Styles.Desc.Render(slot.CapturedAt.Format("2006-01-02 15:04"))
		fileCount := Styles.Desc.Render(fmt.Sprintf("(%d files)", len(slot.Meta.Files)))
		line := cursor + toolLabel + nameLabel + "  " + date + "  " + fileCount
		if s.cursor == i {
			b.WriteString(Styles.Selected.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}
	if end < total {
		b.WriteString(Styles.Desc.Render(fmt.Sprintf("  ↓ %d more below\n", total-end)))
	}
	return b.String()
}

func (s *RestorePickerStep) Name() string { return "Select Backup" }

func (s *RestorePickerStep) Info() StepInfo {
	return StepInfo{
		Name:        "Select Backup",
		KeyBindings: []KeyBinding{BindingsNavigate, BindingsConfirm, BindingsBack, BindingsQuit},
	}
}
