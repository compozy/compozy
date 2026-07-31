package globaldb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

func rejectSessionMetadataWithoutRuntime(ctx context.Context, databasePath string) error {
	root := filepath.Join(filepath.Dir(strings.TrimSpace(databasePath)), "sessions")
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("store: read session metadata directory %q: %w", root, err)
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("store: inspect session metadata before runtime cut: %w", err)
		}
		if !entry.IsDir() {
			continue
		}

		path := store.SessionMetaFile(filepath.Join(root, entry.Name()))
		metadata, readErr := readRuntimeMetadataCut(path)
		if errors.Is(readErr, os.ErrNotExist) {
			continue
		}
		if readErr != nil {
			return readErr
		}
		if !metadata.hasRuntimeStatus {
			return fmt.Errorf(
				"%w: %q must declare runtime_status before opening the global catalog",
				store.ErrSessionMetadataSchemaBehind,
				path,
			)
		}
		if err := store.ValidateSessionRuntime(metadata.status, metadata.transition, metadata.failure); err != nil {
			return fmt.Errorf("store: validate runtime metadata in %q before opening the global catalog: %w", path, err)
		}
	}
	return nil
}

type runtimeMetadataCut struct {
	status           store.SessionRuntimeStatus
	transition       store.SessionRuntimeTransition
	failure          string
	hasRuntimeStatus bool
}

func readRuntimeMetadataCut(path string) (runtimeMetadataCut, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return runtimeMetadataCut{}, fmt.Errorf("store: read session metadata %q before runtime cut: %w", path, err)
	}

	var document map[string]json.RawMessage
	if err := json.Unmarshal(payload, &document); err != nil {
		return runtimeMetadataCut{}, fmt.Errorf("store: decode session metadata %q before runtime cut: %w", path, err)
	}
	status, exists := document["runtime_status"]
	if !exists {
		return runtimeMetadataCut{}, nil
	}

	var result runtimeMetadataCut
	if err := json.Unmarshal(status, &result.status); err != nil {
		return runtimeMetadataCut{}, fmt.Errorf(
			"store: decode runtime status in %q before opening the global catalog: %w",
			path,
			err,
		)
	}
	result.hasRuntimeStatus = true
	if transition, ok := document["runtime_transition"]; ok {
		if err := json.Unmarshal(transition, &result.transition); err != nil {
			return runtimeMetadataCut{}, fmt.Errorf(
				"store: decode runtime transition in %q before opening the global catalog: %w",
				path,
				err,
			)
		}
	}
	if failure, ok := document["runtime_failure"]; ok {
		if err := json.Unmarshal(failure, &result.failure); err != nil {
			return runtimeMetadataCut{}, fmt.Errorf(
				"store: decode runtime failure in %q before opening the global catalog: %w",
				path,
				err,
			)
		}
	}
	return result, nil
}
