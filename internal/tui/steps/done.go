package steps

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"text/template"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/castai/kimchi/internal/tools"
)

//go:embed prompts/welcome.txt
var welcomePromptFS embed.FS

var doneStreamStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Width(60)

type streamChunkMsg struct {
	content string
}

type streamDoneMsg struct{}

type streamStartMsg struct{}

var (
	streamMu      sync.Mutex
	streamCh      chan string
	streamStarted bool
)

type DoneStep struct {
	apiKey      string
	toolIDs     []tools.ToolID
	streamedMsg strings.Builder
	streamDone  bool
	spin        spinner.Model
}

func NewDoneStep(ctx context.Context, apiKey string, toolIDs []tools.ToolID) *DoneStep {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))

	return &DoneStep{
		apiKey:  apiKey,
		toolIDs: toolIDs,
		spin:    sp,
	}
}

func (s *DoneStep) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg { return streamStartMsg{} },
		s.spin.Tick,
	)
}

func (s *DoneStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case streamStartMsg:
		streamMu.Lock()
		if !streamStarted {
			streamStarted = true
			streamCh = make(chan string, 100)
			go s.runStreamBackground()
		}
		streamMu.Unlock()
		return s, s.waitForChunk()

	case streamChunkMsg:
		s.streamedMsg.WriteString(msg.content)
		return s, s.waitForChunk()

	case streamDoneMsg:
		s.streamDone = true
		return s, nil

	case spinner.TickMsg:
		if !s.streamDone {
			var cmd tea.Cmd
			s.spin, cmd = s.spin.Update(msg)
			return s, cmd
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return s, tea.Quit
		case "enter":
			if s.streamDone {
				return s, tea.Quit
			}
		}
	}

	return s, nil
}

func (s *DoneStep) waitForChunk() tea.Cmd {
	return func() tea.Msg {
		streamMu.Lock()
		ch := streamCh
		streamMu.Unlock()

		if ch == nil {
			return streamDoneMsg{}
		}

		content, ok := <-ch
		if !ok {
			return streamDoneMsg{}
		}
		return streamChunkMsg{content: content}
	}
}

func (s *DoneStep) runStreamBackground() {
	defer close(streamCh)

	if s.apiKey == "" {
		s.sendDefaultMessage()
		return
	}

	var toolInfo []string
	for _, toolID := range s.toolIDs {
		if tool, ok := tools.ByID(toolID); ok {
			tip := getToolTip(toolID)
			toolInfo = append(toolInfo, fmt.Sprintf("%s: %s", tool.Name, tip))
		}
	}

	toolsSection := "No specific tools configured."
	if len(toolInfo) > 0 {
		toolsSection = strings.Join(toolInfo, "\n")
	}

	prompt, err := s.buildPrompt(toolsSection)
	if err != nil {
		s.sendDefaultMessage()
		return
	}

	reqBody := map[string]any{
		"model": "glm-5-fp8",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"stream":     true,
		"max_tokens": 500,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		s.sendDefaultMessage()
		return
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", "https://llm.cast.ai/openai/v1/chat/completions", strings.NewReader(string(jsonBody)))
	if err != nil {
		s.sendDefaultMessage()
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.sendDefaultMessage()
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			return
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			content := chunk.Choices[0].Delta.Content
			streamCh <- content
		}
	}
}

func (s *DoneStep) sendDefaultMessage() {
	streamCh <- "Welcome to Cast AI!\\n\\n"
	streamCh <- "You've just unlocked access to powerful open-source models!\n\n"
	streamCh <- "glm-5-fp8 is your reasoning companion for planning,\n"
	streamCh <- "analysis, and solving complex problems.\n\n"
	streamCh <- "minimax-m2.5 is your coding partner for writing,\n"
	streamCh <- "refactoring, and debugging code.\n\n"
	streamCh <- "Don't be shy - experiment boldly! Ask tough questions,\n"
	streamCh <- "request detailed explanations, generate entire features.\n"
	streamCh <- "These models are here to help you build amazing things.\n\n"
	streamCh <- "Enjoy the journey!"
}

func getToolTip(toolID tools.ToolID) string {
	switch toolID {
	case tools.ToolOpenCode:
		return "Run 'opencode' in any project directory to start. Use Ctrl+K for quick actions."
	case tools.ToolClaudeCode:
		return "Run 'claude' to start. Default model is Cast AI's glm-5-fp8. Use /models to switch to Opus/Haiku (actual Claude) if needed."
	case tools.ToolZed:
		return "Open Zed and use Cmd+Enter to send prompts to the AI assistant."
	case tools.ToolCodex:
		return "Run 'codex' with a prompt to generate or modify code directly."
	case tools.ToolCline:
		return "Open VS Code with Cline extension installed and start a new task."
	case tools.ToolGeneric:
		return "Source the exported environment variables in your shell."
	default:
		return "Check the tool's documentation for getting started."
	}
}

func (s *DoneStep) View() string {
	var b strings.Builder

	b.WriteString(Styles.Title.Render("Setup Complete"))
	b.WriteString("\n\n")

	if len(s.toolIDs) > 0 {
		b.WriteString("Configured tools:\n")
		for _, toolID := range s.toolIDs {
			tool, ok := tools.ByID(toolID)
			if ok {
				b.WriteString(fmt.Sprintf("  %s %s\n", Styles.Success.Render("✓"), tool.Name))
			}
		}
		b.WriteString("\n")
	}

	msg := s.streamedMsg.String()
	msg = filterThinking(msg)
	msg = wordWrap(msg, 60)
	b.WriteString(doneStreamStyle.Render(msg))

	if s.streamDone {
		b.WriteString("\n\n")
		b.WriteString(Styles.Help.Render("Press Enter to exit"))
	} else {
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("%s Generating welcome message...", s.spin.View()))
	}

	return b.String()
}

func (s *DoneStep) buildPrompt(toolsSection string) (string, error) {
	tmplContent, err := welcomePromptFS.ReadFile("prompts/welcome.txt")
	if err != nil {
		return "", fmt.Errorf("read welcome prompt: %w", err)
	}

	tmpl, err := template.New("welcome").Parse(string(tmplContent))
	if err != nil {
		return "", fmt.Errorf("parse welcome template: %w", err)
	}

	var buf strings.Builder
	data := map[string]string{"Tools": toolsSection}
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute welcome template: %w", err)
	}

	return buf.String(), nil
}

func wordWrap(text string, width int) string {
	var result strings.Builder
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		if len(line) <= width {
			result.WriteString(line)
			result.WriteString("\n")
			continue
		}

		words := strings.Fields(line)
		if len(words) == 0 {
			result.WriteString("\n")
			continue
		}

		currentLen := 0
		for _, word := range words {
			wordLen := len(word)
			if currentLen == 0 {
				result.WriteString(word)
				currentLen = wordLen
			} else if currentLen+1+wordLen <= width {
				result.WriteString(" ")
				result.WriteString(word)
				currentLen += 1 + wordLen
			} else {
				result.WriteString("\n")
				result.WriteString(word)
				currentLen = wordLen
			}
		}
		result.WriteString("\n")
	}

	return result.String()
}

func filterThinking(s string) string {
	for {
		start := strings.Index(s, "<")
		if start == -1 {
			return s
		}
		end := strings.Index(s[start:], ">")
		if end == -1 {
			return s
		}
		tag := s[start+1 : start+end]
		if tag == "think" || strings.HasPrefix(tag, "think>") {
			closeIdx := strings.Index(s, "</think>")
			if closeIdx == -1 {
				closeIdx = strings.Index(s, ">")
				if closeIdx != -1 && closeIdx > start+end {
					s = s[:start] + s[closeIdx+1:]
					continue
				}
				return s[:start]
			}
			s = s[:start] + s[closeIdx+8:]
			continue
		}
		return s
	}
}

func (s *DoneStep) Name() string {
	return "Done"
}

func (s *DoneStep) Info() StepInfo {
	return StepInfo{
		Name: "Done",
		KeyBindings: []KeyBinding{
			{Key: "↵", Text: "exit"},
		},
	}
}
