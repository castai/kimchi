package tools

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/castai/kimchi/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func testCursorDBPath(t *testing.T) string {
	t.Helper()
	p, err := config.ScopePaths(config.ScopeGlobal, getCursorDBPath())
	require.NoError(t, err)
	return p
}

func createTestCursorDB(t *testing.T, dbPath string) *sql.DB {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(dbPath), 0755))
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB)")
	require.NoError(t, err)
	return db
}

func readCursorStorage(t *testing.T, dbPath string) map[string]any {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var blob []byte
	err = db.QueryRow("SELECT value FROM ItemTable WHERE key = ?", cursorReactiveStorageKey).Scan(&blob)
	require.NoError(t, err)

	var storage map[string]any
	require.NoError(t, json.Unmarshal(blob, &storage))
	return storage
}

func TestWriteCursor_FreshDB(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	dbPath := testCursorDBPath(t)
	db := createTestCursorDB(t, dbPath)
	require.NoError(t, db.Close())

	err := writeCursor(config.ScopeGlobal, "test-api-key")
	require.NoError(t, err)

	storage := readCursorStorage(t, dbPath)

	assert.Equal(t, baseURL, storage["openAIBaseUrl"])
	assert.Equal(t, true, storage["useOpenAIKey"])

	aiSettings, ok := storage["aiSettings"].(map[string]any)
	require.True(t, ok)

	userModels := toStringSlice(aiSettings["userAddedModels"])
	assert.Contains(t, userModels, cursorModelSlug(MainModel))
	assert.Contains(t, userModels, cursorModelSlug(CodingModel))
	assert.Contains(t, userModels, cursorModelSlug(SubModel))

	enabled := toStringSlice(aiSettings["modelOverrideEnabled"])
	assert.Contains(t, enabled, cursorModelSlug(MainModel))
	assert.Contains(t, enabled, cursorModelSlug(CodingModel))
	assert.Contains(t, enabled, cursorModelSlug(SubModel))

	modelConfig, ok := aiSettings["modelConfig"].(map[string]any)
	require.True(t, ok)
	composer, ok := modelConfig["composer"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, cursorModelSlug(MainModel), composer["modelName"])

	rdb, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer rdb.Close() //nolint:errcheck
	var apiKeyVal string
	err = rdb.QueryRow("SELECT value FROM ItemTable WHERE key = ?", cursorAPIKeyRow).Scan(&apiKeyVal)
	require.NoError(t, err)
	assert.Equal(t, "test-api-key", apiKeyVal)
}

func TestWriteCursor_PreservesExistingSettings(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	dbPath := testCursorDBPath(t)
	db := createTestCursorDB(t, dbPath)

	existing := map[string]any{
		"cppEnabled":     true,
		"membershipType": "pro",
		"aiSettings": map[string]any{
			"cmdKModel":             "gpt-4",
			"userAddedModels":       []any{"my-custom-model"},
			"modelOverrideEnabled":  []any{"default", "my-custom-model"},
			"modelOverrideDisabled": []any{cursorModelSlug(MainModel), "some-other-model"},
			"modelConfig": map[string]any{
				"cmd-k": map[string]any{
					"modelName": "gpt-4",
				},
			},
		},
	}
	blob, err := json.Marshal(existing)
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO ItemTable (key, value) VALUES (?, ?)", cursorReactiveStorageKey, string(blob))
	require.NoError(t, err)
	require.NoError(t, db.Close())

	err = writeCursor(config.ScopeGlobal, "test-key")
	require.NoError(t, err)

	storage := readCursorStorage(t, dbPath)

	assert.Equal(t, true, storage["cppEnabled"])
	assert.Equal(t, "pro", storage["membershipType"])

	aiSettings := storage["aiSettings"].(map[string]any)

	assert.Equal(t, "gpt-4", aiSettings["cmdKModel"])

	userModels := toStringSlice(aiSettings["userAddedModels"])
	assert.Contains(t, userModels, "my-custom-model")
	assert.Contains(t, userModels, cursorModelSlug(MainModel))

	disabled := toStringSlice(aiSettings["modelOverrideDisabled"])
	assert.NotContains(t, disabled, cursorModelSlug(MainModel))
	assert.Contains(t, disabled, "some-other-model")

	modelConfig := aiSettings["modelConfig"].(map[string]any)
	cmdK, ok := modelConfig["cmd-k"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, cursorModelSlug(CodingModel), cmdK["modelName"])
	bgComposer, ok := modelConfig["background-composer"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, cursorModelSlug(MainModel), bgComposer["modelName"])
	quickAgent, ok := modelConfig["quick-agent"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, cursorModelSlug(SubModel), quickAgent["modelName"])
}

func TestWriteCursor_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	dbPath := testCursorDBPath(t)
	db := createTestCursorDB(t, dbPath)
	require.NoError(t, db.Close())

	require.NoError(t, writeCursor(config.ScopeGlobal, "key1"))
	require.NoError(t, writeCursor(config.ScopeGlobal, "key2"))

	storage := readCursorStorage(t, dbPath)
	aiSettings := storage["aiSettings"].(map[string]any)

	userModels := toStringSlice(aiSettings["userAddedModels"])
	count := 0
	for _, m := range userModels {
		if m == cursorModelSlug(MainModel) {
			count++
		}
	}
	assert.Equal(t, 1, count, "model should appear exactly once after two writes")

	rdb, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer rdb.Close() //nolint:errcheck
	var apiKeyVal string
	require.NoError(t, rdb.QueryRow("SELECT value FROM ItemTable WHERE key = ?", cursorAPIKeyRow).Scan(&apiKeyVal))
	assert.Equal(t, "key2", apiKeyVal)
}

func TestWriteCursor_StoresAsText(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	dbPath := testCursorDBPath(t)
	db := createTestCursorDB(t, dbPath)
	require.NoError(t, db.Close())

	require.NoError(t, writeCursor(config.ScopeGlobal, "test-key"))

	rdb, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer rdb.Close() //nolint:errcheck

	var valType string
	err = rdb.QueryRow("SELECT typeof(value) FROM ItemTable WHERE key = ?", cursorReactiveStorageKey).Scan(&valType)
	require.NoError(t, err)
	assert.Equal(t, "text", valType, "value must be stored as text, not blob")

	err = rdb.QueryRow("SELECT typeof(value) FROM ItemTable WHERE key = ?", cursorAPIKeyRow).Scan(&valType)
	require.NoError(t, err)
	assert.Equal(t, "text", valType, "API key must be stored as text")
}

func TestWriteCursor_EmptyAPIKey(t *testing.T) {
	err := writeCursor(config.ScopeGlobal, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API key not configured")
}

func TestWriteCursor_MissingDB(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	err := writeCursor(config.ScopeGlobal, "test-key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
