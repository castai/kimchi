package update

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractStructuredArchive_SplitsBinAndData(t *testing.T) {
	binaryContent := []byte("#!/bin/sh\necho hello")
	packageJSON := []byte(`{"name":"test","version":"1.0.0"}`)
	themeContent := []byte(`{"background":"#000"}`)

	archive := createArchive(t, []archiveFile{
		{Name: "bin", IsDir: true},
		{Name: "bin/kimchi-code", Content: binaryContent, Mode: 0755},
		{Name: "share/kimchi", IsDir: true},
		{Name: "share/kimchi/package.json", Content: packageJSON},
		{Name: "share/kimchi/theme", IsDir: true},
		{Name: "share/kimchi/theme/dark.json", Content: themeContent},
	})

	root, err := extractStructuredArchive(bytes.NewReader(archive), "kimchi-code")
	require.NoError(t, err)
	defer os.RemoveAll(root)

	gotBinary, err := os.ReadFile(filepath.Join(root, "bin", "kimchi-code"))
	require.NoError(t, err)
	assert.Equal(t, binaryContent, gotBinary)

	gotPkg, err := os.ReadFile(filepath.Join(root, "share", "kimchi", "package.json"))
	require.NoError(t, err)
	assert.Equal(t, packageJSON, gotPkg)

	gotTheme, err := os.ReadFile(filepath.Join(root, "share", "kimchi", "theme", "dark.json"))
	require.NoError(t, err)
	assert.Equal(t, themeContent, gotTheme)
}

func TestExtractStructuredArchive_MissingBinary(t *testing.T) {
	archive := createArchive(t, []archiveFile{
		{Name: "share/kimchi/package.json", Content: []byte(`{}`)},
	})

	_, err := extractStructuredArchive(bytes.NewReader(archive), "kimchi-code")
	require.Error(t, err)
	assert.ErrorContains(t, err, "kimchi-code")
	assert.ErrorContains(t, err, "not found")
}

func TestExtractStructuredArchive_PreservesFilePermissions(t *testing.T) {
	archive := createArchive(t, []archiveFile{
		{Name: "bin/kimchi-code", Content: []byte("binary"), Mode: 0755},
		{Name: "share/kimchi/package.json", Content: []byte(`{}`)},
	})

	root, err := extractStructuredArchive(bytes.NewReader(archive), "kimchi-code")
	require.NoError(t, err)
	defer os.RemoveAll(root)

	info, err := os.Stat(filepath.Join(root, "bin", "kimchi-code"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), info.Mode().Perm())
}

func TestExtractStructuredArchive_SkipsDirectoryTraversal(t *testing.T) {
	archive := createArchive(t, []archiveFile{
		{Name: "bin/kimchi-code", Content: []byte("binary"), Mode: 0755},
		{Name: "share/kimchi/package.json", Content: []byte(`{}`)},
		{Name: "../etc/passwd", Content: []byte("malicious")},
	})

	root, err := extractStructuredArchive(bytes.NewReader(archive), "kimchi-code")
	require.NoError(t, err)
	defer os.RemoveAll(root)

	assert.FileExists(t, filepath.Join(root, "bin", "kimchi-code"))
	assert.FileExists(t, filepath.Join(root, "share", "kimchi", "package.json"))
	assert.NoFileExists(t, filepath.Join(root, "..", "etc", "passwd"))
}
