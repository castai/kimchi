package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadJSON reads a JSON or JSONC (JSON with Comments) file and returns its
// contents as a map. If the exact path does not exist and has a ".json"
// extension, it also tries the corresponding ".jsonc" path, since tools like
// OpenCode and get-shit-done-cc prefer the .jsonc variant.
func ReadJSON(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Try the .jsonc sibling when the .json file is absent.
			if strings.HasSuffix(path, ".json") {
				jsoncPath := path + "c"
				data, err = os.ReadFile(jsoncPath)
				if err != nil {
					if os.IsNotExist(err) {
						return make(map[string]any), nil
					}
					return nil, fmt.Errorf("read file: %w", err)
				}
			} else {
				return make(map[string]any), nil
			}
		} else {
			return nil, fmt.Errorf("read file: %w", err)
		}
	}

	stripped := stripJSONCComments(data)

	var result map[string]any
	if err := json.Unmarshal(stripped, &result); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	// Literal `null` unmarshals to a nil map; normalize so callers can
	// always delete/insert without a nil check.
	if result == nil {
		result = make(map[string]any)
	}

	return result, nil
}

// stripJSONCComments removes single-line (//) and block (/* */) comments from
// JSON content so that JSONC files can be parsed by encoding/json. String
// literals are handled correctly — slashes inside strings are left alone.
func stripJSONCComments(data []byte) []byte {
	out := make([]byte, 0, len(data))
	inString := false
	i := 0
	for i < len(data) {
		c := data[i]
		if inString {
			out = append(out, c)
			if c == '\\' && i+1 < len(data) {
				// Escaped character — copy the next byte verbatim and skip it.
				i++
				out = append(out, data[i])
			} else if c == '"' {
				inString = false
			}
			i++
			continue
		}
		// Outside a string.
		if c == '"' {
			inString = true
			out = append(out, c)
			i++
			continue
		}
		if c == '/' && i+1 < len(data) {
			next := data[i+1]
			if next == '/' {
				// Single-line comment: skip until newline.
				i += 2
				for i < len(data) && data[i] != '\n' {
					i++
				}
				continue
			}
			if next == '*' {
				// Block comment: skip until closing */.
				i += 2
				for i+1 < len(data) {
					if data[i] == '*' && data[i+1] == '/' {
						i += 2
						break
					}
					i++
				}
				continue
			}
		}
		out = append(out, c)
		i++
	}
	return out
}

func WriteJSON(path string, data map[string]any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, jsonData, 0600); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename file: %w", err)
	}

	return nil
}

func WriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename file: %w", err)
	}

	return nil
}
