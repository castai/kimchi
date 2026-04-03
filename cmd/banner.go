package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/castai/kimchi/internal/config"
	"github.com/castai/kimchi/internal/tools"
)

const (
	ansiReset = "\033[0m"
	ansiRed   = "\033[38;5;196m"
	ansiBold  = "\033[1m"
	ansiDim   = "\033[2m"
)

func colorEnabled() bool {
	if _, set := os.LookupEnv("NO_COLOR"); set {
		return false
	}
	// Windows Terminal and modern PowerShell support ANSI but don't set TERM.
	if os.Getenv("WT_SESSION") != "" || os.Getenv("COLORTERM") != "" {
		return true
	}
	term := os.Getenv("TERM")
	return term != "" && term != "dumb"
}

func printBanner(w io.Writer, wrapping string, cfg *config.Config) {
	line := strings.Repeat("\u2500", 45)
	models := fmt.Sprintf("%s (reasoning) / %s (coding)", tools.MainModel.Slug, tools.CodingModel.Slug)
	gsdStatus := "not installed"
	for _, t := range cfg.GSDInstalledFor {
		if strings.Contains(t, wrapping) {
			gsdStatus = "active"
			break
		}
	}

	if colorEnabled() {
		fmt.Fprintf(w, "\n  %s%s\U0001F96C\U0001F336  kimchi%s\n", ansiBold, ansiRed, ansiReset)
		fmt.Fprintf(w, "  %s%s%s\n", ansiDim, line, ansiReset)
		fmt.Fprintf(w, "  %sTarget:%s  %s\n", ansiDim, ansiReset, wrapping)
		fmt.Fprintf(w, "  %sModels:%s  %s\n", ansiDim, ansiReset, models)
		fmt.Fprintf(w, "  %sGSD:%s     %s\n", ansiDim, ansiReset, gsdStatus)
		fmt.Fprintf(w, "  %sMode:%s    %s\n", ansiDim, ansiReset, cfg.Mode)
		fmt.Fprintf(w, "  %s%s%s\n\n", ansiDim, line, ansiReset)
	} else {
		fmt.Fprintf(w, "\n  kimchi\n")
		fmt.Fprintf(w, "  %s\n", strings.Repeat("-", 45))
		fmt.Fprintf(w, "  Target:  %s\n", wrapping)
		fmt.Fprintf(w, "  Models:  %s\n", models)
		fmt.Fprintf(w, "  GSD:     %s\n", gsdStatus)
		fmt.Fprintf(w, "  Mode:    %s\n", cfg.Mode)
		fmt.Fprintf(w, "  %s\n\n", strings.Repeat("-", 45))
	}
}
