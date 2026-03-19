package steps

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Styles contains all shared styling for wizard steps.
var Styles = struct {
	Border   lipgloss.Style
	StepName lipgloss.Style

	Key  lipgloss.Style
	Help lipgloss.Style

	Title    lipgloss.Style
	Item     lipgloss.Style
	Selected lipgloss.Style
	Cursor   lipgloss.Style
	Desc     lipgloss.Style

	Success lipgloss.Style
	Error   lipgloss.Style
	Spinner lipgloss.Style
	Pending lipgloss.Style
	Warning lipgloss.Style
}{
	Border:   lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
	StepName: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")),

	Key:  lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
	Help: lipgloss.NewStyle().Foreground(lipgloss.Color("8")),

	Title:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")),
	Item:     lipgloss.NewStyle().PaddingLeft(2),
	Selected: lipgloss.NewStyle().Foreground(lipgloss.Color("14")),
	Cursor:   lipgloss.NewStyle().Foreground(lipgloss.Color("14")),
	Desc:     lipgloss.NewStyle().Foreground(lipgloss.Color("8")),

	Success: lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
	Error:   lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
	Spinner: lipgloss.NewStyle().Foreground(lipgloss.Color("12")),
	Pending: lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
	Warning: lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
}

// KeyBinding represents a keyboard shortcut displayed in the footer.
type KeyBinding struct {
	Key  string
	Text string
}

// Standard key bindings for reuse across steps.
var (
	BindingsQuit     = KeyBinding{Key: "q", Text: "quit"}
	BindingsBack     = KeyBinding{Key: "esc", Text: "back"}
	BindingsConfirm  = KeyBinding{Key: "↵", Text: "confirm"}
	BindingsNavigate = KeyBinding{Key: "↑/↓", Text: "navigate"}
	BindingsSelect   = KeyBinding{Key: "space", Text: "select"}
)

// StepInfo provides metadata for step rendering.
type StepInfo struct {
	Name        string
	KeyBindings []KeyBinding
}

// Header renders the step header with progress dots.
func Header(name string) string {
	bar := Styles.Border.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	nameText := Styles.StepName.Render(name)

	return fmt.Sprintf("%s\n %s\n%s\n\n", bar, nameText, bar)
}

// Footer renders the key binding hints.
func Footer(bindings []KeyBinding) string {
	if len(bindings) == 0 {
		return ""
	}
	var parts []string
	for _, b := range bindings {
		parts = append(parts, fmt.Sprintf("%s %s", Styles.Key.Render(b.Key), Styles.Help.Render(b.Text)))
	}
	return "\n\n" + Styles.Help.Render(strings.Join(parts, "  "))
}

// StepView renders a complete step with header + content + footer.
func StepView(info StepInfo, content string) string {
	return Header(info.Name) + content + Footer(info.KeyBindings)
}

// ProgressDots returns just the dot string (e.g., "●●○○○").
func Hyperlink(url, text string) string {
	return termenv.Hyperlink(url, text)
}
