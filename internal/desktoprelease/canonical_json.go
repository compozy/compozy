package desktoprelease

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func writeCanonicalJSON(path string, value any) error {
	contents, err := canonicalJSON(value)
	if err != nil {
		return fmt.Errorf("desktop release: encode canonical JSON: %w", err)
	}
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		return fmt.Errorf("desktop release: write %s: %w", filepath.Base(path), err)
	}
	return nil
}

func canonicalJSON(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return CanonicalizeJSON(encoded)
}

// CanonicalizeJSON returns one JSON document with recursively sorted object keys.
func CanonicalizeJSON(contents []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.UseNumber()
	var document any
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("multiple json documents")
		}
		return nil, err
	}
	return json.Marshal(document)
}
