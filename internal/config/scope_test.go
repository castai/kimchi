package config

import (
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
			name:  "project creates .claude directory",
			scope: ScopeProject,
			input: "~/.config/claude/settings.json",
			check: func(t *testing.T, result string) {
				if !strings.HasSuffix(result, ".claude/settings.json") {
					t.Errorf("expected .claude/settings.json, got %s", result)
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
