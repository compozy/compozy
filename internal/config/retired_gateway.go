package config

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	burnttoml "github.com/BurntSushi/toml"
	"github.com/compozy/compozy/internal/fileutil"
)

const retiredGatewayKey = "gateway"

// MigrateRemovedGateway records the retired profiles before publishing the cleaned config.
func MigrateRemovedGateway(path string, record func(int) error) (_ []string, err error) {
	configOverlayWriteMu.Lock()
	defer configOverlayWriteMu.Unlock()

	original, exists, err := readOptionalRegularFile(path, "config overlay")
	if err != nil || !exists {
		return nil, err
	}
	clean, keys, count, err := stripRemovedGateway(original)
	if err != nil || len(keys) == 0 {
		return nil, err
	}
	directory, name, err := fileutil.OpenParentDirectory(path)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, directory.Close()) }()
	if err := requireOverlayContents(directory, name, path, original, exists); err != nil {
		return nil, err
	}
	if count > 0 {
		if err := record(count); err != nil {
			return nil, fmt.Errorf("record removed connection profiles: %w", err)
		}
	}
	if err := writePersistedFileInDirectory(directory, name, path, clean, true); err != nil {
		return nil, err
	}
	return keys, nil
}

func stripRemovedGateway(contents []byte) ([]byte, []string, int, error) {
	var values map[string]any
	meta, err := burnttoml.Decode(string(contents), &values)
	if err != nil {
		// Let the owning decoder report errors with the normal source and position.
		return contents, nil, 0, nil
	}
	gateway, exists := values[retiredGatewayKey]
	if !exists {
		return contents, nil, 0, nil
	}
	document, err := parseOverlayDocument(contents)
	if err != nil {
		return nil, nil, 0, err
	}
	for _, expr := range slices.Backward(document.expressions) {
		if pathHasPrefix(expr.path, []string{retiredGatewayKey}) {
			contents = replaceOffsets(contents, rangeStart(expr.raw), rangeEnd(expr.raw), nil)
		}
	}

	keys := make(map[string]struct{})
	for _, key := range meta.Keys() {
		if len(key) > 1 && key[0] == retiredGatewayKey {
			if key[1] == "connections" {
				key = key[:2]
			}
			keys[key.String()] = struct{}{}
		}
	}
	if len(keys) == 0 {
		keys[retiredGatewayKey] = struct{}{}
	}
	var count int
	if table, ok := gateway.(map[string]any); ok {
		switch connections := table["connections"].(type) {
		case []map[string]any:
			count = len(connections)
		case []any:
			count = len(connections)
		}
	}
	return contents, slices.Sorted(maps.Keys(keys)), count, nil
}
