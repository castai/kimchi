package update

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHarnessPathInDir(t *testing.T) {
	got := HarnessPathInDir("/usr/local/bin")
	assert.Equal(t, "/usr/local/bin/kimchi_code", got)
}

func Test_HarnessInstalled(t *testing.T) {
	t.Run("exists", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "kimchi_code")
		require.NoError(t, os.WriteFile(path, []byte("binary"), 0755))
		assert.True(t, HarnessInstalled(path))
	})

	t.Run("missing", func(t *testing.T) {
		assert.False(t, HarnessInstalled(filepath.Join(t.TempDir(), "kimchi_code")))
	})
}

func Test_harnessVersion(t *testing.T) {
	t.Run("parses version from output", func(t *testing.T) {
		dir := t.TempDir()
		bin := filepath.Join(dir, "kimchi_code")
		require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\necho 'kimchi_code 0.5.0'"), 0755))

		v, err := harnessVersion(context.Background(), bin)
		require.NoError(t, err)
		assert.Equal(t, "v0.5.0", v)
	})

	t.Run("parses version with v prefix", func(t *testing.T) {
		dir := t.TempDir()
		bin := filepath.Join(dir, "kimchi_code")
		require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\necho 'kimchi_code v1.2.3'"), 0755))

		v, err := harnessVersion(context.Background(), bin)
		require.NoError(t, err)
		assert.Equal(t, "v1.2.3", v)
	})

	t.Run("parses version from multiline output", func(t *testing.T) {
		dir := t.TempDir()
		bin := filepath.Join(dir, "kimchi_code")
		require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\necho 'kimchi_code'\necho 'version: 2.0.1'\necho 'build: abc123'"), 0755))

		v, err := harnessVersion(context.Background(), bin)
		require.NoError(t, err)
		assert.Equal(t, "v2.0.1", v)
	})

	t.Run("error on missing binary", func(t *testing.T) {
		_, err := harnessVersion(context.Background(), filepath.Join(t.TempDir(), "nonexistent"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "run nonexistent version")
	})

	t.Run("error on garbage output", func(t *testing.T) {
		dir := t.TempDir()
		bin := filepath.Join(dir, "kimchi_code")
		require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\necho 'no version here'"), 0755))

		_, err := harnessVersion(context.Background(), bin)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no semver found")
	})

	t.Run("error on non-zero exit", func(t *testing.T) {
		dir := t.TempDir()
		bin := filepath.Join(dir, "kimchi_code")
		require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\nexit 1"), 0755))

		_, err := harnessVersion(context.Background(), bin)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "run kimchi_code version")
	})

}
