package steps

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// SeedLookupFn returns pre-fill values for author, description, and tags
// given a recipe name. Called when the cursor leaves the name field.
// Return empty strings / nil slice to leave fields as-is.
type SeedLookupFn func(name string) (author, description string, tags []string)

// ExportMetaStep collects recipe metadata: name, author, description.
type ExportMetaStep struct {
	inputs       [3]textinput.Model
	focused      int
	err          string
	seedLookupFn SeedLookupFn
}

// NewExportMetaStep creates the metadata step. Initial values pre-fill the
// corresponding fields; empty strings leave them blank.
func NewExportMetaStep(initialName, initialAuthor, initialDescription string) *ExportMetaStep {
	labels := []string{"Recipe name", "Author", "Description (optional)"}
	s := &ExportMetaStep{}
	for i, label := range labels {
		ti := textinput.New()
		ti.Placeholder = label
		ti.Width = 50
		s.inputs[i] = ti
	}
	s.inputs[0].SetValue(initialName)
	s.inputs[1].SetValue(initialAuthor)
	s.inputs[2].SetValue(initialDescription)
	s.inputs[0].Focus()
	return s
}

// SetSeedLookupFn attaches a function that pre-fills author/description/tags
// when the user moves away from the name field.
func (s *ExportMetaStep) SetSeedLookupFn(fn SeedLookupFn) {
	s.seedLookupFn = fn
}

func (s *ExportMetaStep) RecipeName() string   { return strings.TrimSpace(s.inputs[0].Value()) }
func (s *ExportMetaStep) Author() string        { return strings.TrimSpace(s.inputs[1].Value()) }
func (s *ExportMetaStep) Description() string   { return strings.TrimSpace(s.inputs[2].Value()) }

func (s *ExportMetaStep) Init() tea.Cmd {
	return textinput.Blink
}

// applySeedIfLeavingName pre-fills author/description from the seed lookup
// function when the cursor is leaving field 0 (the name field).
func (s *ExportMetaStep) applySeedIfLeavingName() {
	if s.focused != 0 || s.seedLookupFn == nil {
		return
	}
	name := s.RecipeName()
	if name == "" {
		return
	}
	author, description, _ := s.seedLookupFn(name)
	if s.inputs[1].Value() == "" && author != "" {
		s.inputs[1].SetValue(author)
	}
	if s.inputs[2].Value() == "" && description != "" {
		s.inputs[2].SetValue(description)
	}
}

func (s *ExportMetaStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return s, func() tea.Msg { return AbortMsg{} }
		case "esc":
			if s.focused > 0 {
				s.inputs[s.focused].Blur()
				s.focused--
				s.inputs[s.focused].Focus()
				return s, textinput.Blink
			}
			return s, func() tea.Msg { return PrevStepMsg{} }
		case "tab", "down":
			s.applySeedIfLeavingName()
			s.inputs[s.focused].Blur()
			s.focused = (s.focused + 1) % len(s.inputs)
			s.inputs[s.focused].Focus()
			return s, textinput.Blink
		case "shift+tab", "up":
			s.inputs[s.focused].Blur()
			s.focused = (s.focused - 1 + len(s.inputs)) % len(s.inputs)
			s.inputs[s.focused].Focus()
			return s, textinput.Blink
		case "enter":
			if s.focused < len(s.inputs)-1 {
				// Advance to next field.
				s.applySeedIfLeavingName()
				s.inputs[s.focused].Blur()
				s.focused++
				s.inputs[s.focused].Focus()
				return s, textinput.Blink
			}
			// Last field — validate required fields and advance step.
			if s.RecipeName() == "" {
				s.err = "Recipe name is required"
				s.inputs[s.focused].Blur()
				s.focused = 0
				s.inputs[s.focused].Focus()
				return s, textinput.Blink
			}
			if s.Author() == "" {
				s.err = "Author is required"
				s.inputs[s.focused].Blur()
				s.focused = 1
				s.inputs[s.focused].Focus()
				return s, textinput.Blink
			}
			s.err = ""
			return s, func() tea.Msg { return NextStepMsg{} }
		}
	}

	var cmd tea.Cmd
	s.inputs[s.focused], cmd = s.inputs[s.focused].Update(msg)
	return s, cmd
}

func (s *ExportMetaStep) View() string {
	var b strings.Builder

	b.WriteString("Provide metadata for the recipe file.\n\n")

	labels := []string{"Name", "Author", "Description"}
	for i, label := range labels {
		cursor := "  "
		if s.focused == i {
			cursor = Styles.Cursor.Render("► ")
		}
		b.WriteString(cursor + Styles.Desc.Render(label+":\n"))
		b.WriteString("  " + s.inputs[i].View())
		b.WriteString("\n\n")
	}

	if s.err != "" {
		b.WriteString(Styles.Error.Render("✗ " + s.err))
		b.WriteString("\n")
	}

	return b.String()
}

func (s *ExportMetaStep) Name() string { return "Recipe Metadata" }

func (s *ExportMetaStep) Info() StepInfo {
	return StepInfo{
		Name: "Recipe Metadata",
		KeyBindings: []KeyBinding{
			{Key: "tab", Text: "next field"},
			BindingsConfirm,
			BindingsBack,
			BindingsQuit,
		},
	}
}
