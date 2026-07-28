package spec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Render renders the canonical OpenAPI document as deterministic JSON bytes.
func Render() ([]byte, error) {
	doc, err := Document()
	if err != nil {
		return nil, err
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal openapi: %w", err)
	}
	data = append(data, '\n')
	return data, nil
}

// WriteFile renders the canonical OpenAPI document to the supplied path.
func WriteFile(path string) error {
	data, err := Render()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
