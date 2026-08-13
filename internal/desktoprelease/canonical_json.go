package desktoprelease

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var document any
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	return json.Marshal(document)
}
