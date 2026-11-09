package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/BurntSushi/toml"
)

// liveOverride is one config value live mode replaced and must hand back.
type liveOverride struct {
	table    []string
	key      string
	injected any
	previous any
	existed  bool
}

func lookupTable(root map[string]any, keys ...string) (map[string]any, bool) {
	current := root
	for _, key := range keys {
		next, ok := current[key].(map[string]any)
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func encodeConfig(values map[string]any) ([]byte, error) {
	var encoded bytes.Buffer
	if err := toml.NewEncoder(&encoded).Encode(values); err != nil {
		return nil, fmt.Errorf("demo seed: encode config: %w", err)
	}
	return encoded.Bytes(), nil
}

func decodeConfig(raw []byte) (map[string]any, error) {
	values := map[string]any{}
	if _, err := toml.Decode(string(raw), &values); err != nil {
		return nil, fmt.Errorf("demo seed: parse config: %w", err)
	}
	return values, nil
}

// applyLiveConfig writes the overrides and returns the undo. The undo keeps anything the
// operator changed while live mode ran, including a later edit to an overridden key.
func applyLiveConfig(path string, overrides []liveOverride) (func() error, error) {
	// #nosec G304,G703 -- the seeded home's own config file.
	original, readErr := os.ReadFile(path)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return nil, fmt.Errorf("demo seed: read config: %w", readErr)
	}
	values, err := decodeConfig(original)
	if err != nil {
		return nil, err
	}
	for index := range overrides {
		table := nestedTable(values, overrides[index].table...)
		overrides[index].previous, overrides[index].existed = table[overrides[index].key]
		table[overrides[index].key] = overrides[index].injected
	}
	written, err := encodeConfig(values)
	if err != nil {
		return nil, err
	}
	// #nosec G703 -- the seeded home's own config file.
	if err := os.WriteFile(path, written, 0o600); err != nil {
		return nil, fmt.Errorf("demo seed: write config: %w", err)
	}
	return func() error {
		return restoreLiveConfig(path, original, readErr == nil, written, overrides)
	}, nil
}

func restoreLiveConfig(path string, original []byte, existed bool, written []byte, overrides []liveOverride) error {
	// #nosec G304,G703 -- the seeded home's own config file.
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("demo seed: read config to restore: %w", err)
	}
	if bytes.Equal(current, written) {
		if !existed {
			// #nosec G703 -- the seeded home's own config file.
			return os.Remove(path)
		}
		// #nosec G703 -- the seeded home's own config file.
		return os.WriteFile(path, original, 0o600)
	}
	values, err := decodeConfig(current)
	if err != nil {
		return err
	}
	for _, override := range overrides {
		table, ok := lookupTable(values, override.table...)
		if !ok || !reflect.DeepEqual(table[override.key], override.injected) {
			continue
		}
		if override.existed {
			table[override.key] = override.previous
		} else {
			delete(table, override.key)
		}
	}
	restored, err := encodeConfig(values)
	if err != nil {
		return err
	}
	// #nosec G703 -- the seeded home's own config file.
	return os.WriteFile(path, restored, 0o600)
}
