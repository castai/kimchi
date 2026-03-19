package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestScopePaths(t *testing.T) {
	tests := []struct {
		name  string
		scope ConfigScope
		input string
		check func(t *testing.T, result string)
	}{
		{
			name:  "global with tilde",
			scope: ScopeGlobal,
			input: "~/.config/opencode/opencode.json",
			check: func(t *testing.T, result string) {
				if !strings.HasSuffix(result, ".config/opencode/opencode.json") {
					t.Errorf("expected path ending with .config/opencode/opencode.json, got %s", result)
				}
			},
		},
		{
			name:  "project creates dotfile",
			scope: ScopeProject,
			input: "~/.config/opencode/opencode.json",
			check: func(t *testing.T, result string) {
				filename := filepath.Base(result)
				if !strings.HasPrefix(filename, ".") {
					t.Errorf("expected dotfile, got %s", filename)
				}
			},
		},
		{
			name:  "project doesn't double dot prefix",
			scope: ScopeProject,
			input: "~/.opencode.json",
			check: func(t *testing.T, result string) {
				filename := filepath.Base(result)
				if strings.Count(filename, ".") > 1 && strings.HasPrefix(filename, "..") {
					t.Errorf("expected single dot prefix, got %s", filename)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ScopePaths(tt.scope, tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.check(t, result)
		})
	}
}
