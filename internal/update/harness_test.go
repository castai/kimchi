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
