package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/castai/kimchi/internal/config"
	"github.com/castai/kimchi/internal/tools"
	"github.com/stretchr/testify/assert"
)

func testConfig() *config.Config {
	return &config.Config{
		Mode:            config.ModeInject,
		GSDInstalledFor: []string{"opencode"},
	}
}

func testModelConfig() tools.ModelConfig {
	main := tools.Model{Slug: "kimchi-main", DisplayName: "Kimchi Main", Reasoning: true, ToolCall: true}
	coding := tools.Model{Slug: "kimchi-coding", DisplayName: "Kimchi Coding", ToolCall: true}
	sub := tools.Model{Slug: "kimchi-sub", DisplayName: "Kimchi Sub", ToolCall: true}
	return tools.ModelConfig{
		Main:   main,
		Coding: coding,
		Sub:    sub,
		All:    []tools.Model{main, coding, sub},
	}
}

func TestPrintBanner_ContainsKimchi(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	var buf bytes.Buffer
	printBanner(&buf, "opencode", testConfig(), testModelConfig())

	output := buf.String()
	assert.Contains(t, output, "kimchi")
	assert.Contains(t, output, "opencode")
}

func TestPrintBanner_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "")

	var buf bytes.Buffer
	printBanner(&buf, "opencode", testConfig(), testModelConfig())

	output := buf.String()
	assert.NotContains(t, output, "\033[", "output should not contain ANSI escape codes when NO_COLOR is set")
}

func TestPrintBanner_ColorEnabled(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	t.Cleanup(func() { os.Unsetenv("NO_COLOR") })
	t.Setenv("TERM", "xterm-256color")

	var buf bytes.Buffer
	printBanner(&buf, "opencode", testConfig(), testModelConfig())

	output := buf.String()
	assert.Contains(t, output, "\033[", "output should contain ANSI escape codes when color is enabled")
	assert.Contains(t, output, "opencode")
}

func TestPrintBanner_ShowsModels(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	var buf bytes.Buffer
	printBanner(&buf, "opencode", testConfig(), testModelConfig())

	output := buf.String()
	assert.Contains(t, output, "Models:")
	assert.Contains(t, output, "reasoning")
	assert.Contains(t, output, "coding")
}

func TestPrintBanner_ShowsGSDActive(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	var buf bytes.Buffer
	printBanner(&buf, "opencode", testConfig(), testModelConfig())

	assert.Contains(t, buf.String(), "active")
}

func TestPrintBanner_ShowsGSDNotInstalled(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	cfg := &config.Config{Mode: config.ModeInject}
	var buf bytes.Buffer
	printBanner(&buf, "codex", cfg, testModelConfig())

	assert.Contains(t, buf.String(), "not installed")
}

func TestPrintBanner_OutputEndsWithNewline(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	var buf bytes.Buffer
	printBanner(&buf, "opencode", testConfig(), testModelConfig())

	output := buf.String()
	assert.True(t, strings.HasSuffix(output, "\n"), "banner output should end with a newline")
}
