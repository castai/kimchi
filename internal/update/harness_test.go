package update

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHarnessPathInDir(t *testing.T) {
	got := HarnessPathInDir("/usr/local/bin")
	assert.Equal(t, "/usr/local/bin/kimchi-code", got)
}

func Test_HarnessInstalled(t *testing.T) {
	t.Run("exists", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "kimchi-code")
		require.NoError(t, os.WriteFile(path, []byte("binary"), 0755))
		assert.True(t, HarnessInstalled(path))
	})

	t.Run("missing", func(t *testing.T) {
		assert.False(t, HarnessInstalled(filepath.Join(t.TempDir(), "kimchi-code")))
	})
}

func TestResolveHarnessPackageJSON(t *testing.T) {
	t.Run("prefers XDG data dir when package.json exists there", func(t *testing.T) {
		dataDir := t.TempDir()
		t.Setenv("XDG_DATA_HOME", dataDir)

		xdgPkg := filepath.Join(dataDir, "kimchi", "package.json")
		require.NoError(t, os.MkdirAll(filepath.Dir(xdgPkg), 0755))
		require.NoError(t, os.WriteFile(xdgPkg, []byte(`{"version":"2.0.0"}`), 0644))

		binDir := t.TempDir()
		binaryPath := filepath.Join(binDir, "kimchi-code")
		legacyPkg := filepath.Join(binDir, "package.json")
		require.NoError(t, os.WriteFile(legacyPkg, []byte(`{"version":"1.0.0"}`), 0644))

		got, err := resolveHarnessPackageJSON(binaryPath)
		require.NoError(t, err)
		assert.Equal(t, xdgPkg, got)
	})

	t.Run("falls back to next to binary when XDG data dir has no package.json", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", t.TempDir())

		binDir := t.TempDir()
		binaryPath := filepath.Join(binDir, "kimchi-code")

		got, err := resolveHarnessPackageJSON(binaryPath)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(binDir, "package.json"), got)
	})
}
