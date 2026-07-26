package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/agh/internal/fileutil"
)

// ReadSessionMeta loads a session metadata document from disk.
func ReadSessionMeta(path string) (SessionMeta, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return SessionMeta{}, errors.New("store: session meta path is required")
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return SessionMeta{}, fmt.Errorf("store: read session meta %q: %w", cleanPath, err)
	}

	var meta SessionMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return SessionMeta{}, fmt.Errorf("store: decode session meta %q: %w", cleanPath, err)
	}
	if err := meta.Validate(); err != nil {
		return SessionMeta{}, err
	}

	return meta, nil
}

// WriteSessionMeta writes the metadata file atomically via temp file and rename.
func WriteSessionMeta(path string, meta SessionMeta) error {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return errors.New("store: session meta path is required")
	}
	if err := meta.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o755); err != nil {
		return fmt.Errorf("store: create session meta directory for %q: %w", cleanPath, err)
	}

	payload, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("store: encode session meta %q: %w", cleanPath, err)
	}
	payload = append(payload, '\n')

	if err := fileutil.AtomicWriteFile(cleanPath, payload, 0o644); err != nil {
		return fmt.Errorf("store: write session meta %q: %w", cleanPath, err)
	}

	return nil
}
