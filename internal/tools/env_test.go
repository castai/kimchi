package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeEnv(t *testing.T) {
	t.Run("adds new variables", func(t *testing.T) {
		result := MergeEnv(map[string]string{"NEW_VAR": "value"})
		found := false
		for _, e := range result {
			if e == "NEW_VAR=value" {
				found = true
				break
			}
		}
		assert.True(t, found, "NEW_VAR should be in merged env")
	})

	t.Run("overrides existing variables", func(t *testing.T) {
		t.Setenv("TEST_OVERRIDE", "old")
		result := MergeEnv(map[string]string{"TEST_OVERRIDE": "new"})

		count := 0
		for _, e := range result {
			if e == "TEST_OVERRIDE=new" {
				count++
			}
			if e == "TEST_OVERRIDE=old" {
				t.Fatal("old value should not be present")
			}
		}
		assert.Equal(t, 1, count, "override should appear exactly once")
	})

	t.Run("preserves unrelated variables", func(t *testing.T) {
		t.Setenv("KEEP_ME", "preserved")
		result := MergeEnv(map[string]string{"OTHER": "value"})

		found := false
		for _, e := range result {
			if e == "KEEP_ME=preserved" {
				found = true
				break
			}
		}
		assert.True(t, found, "unrelated env vars should be preserved")
	})

	t.Run("empty overrides returns current env", func(t *testing.T) {
		result := MergeEnv(map[string]string{})
		assert.NotEmpty(t, result, "should return current environment")
	})
}

func TestExitError(t *testing.T) {
	err := &ExitError{Code: 42}
	assert.Equal(t, "exit status 42", err.Error())
	assert.Equal(t, 42, err.Code)
}
