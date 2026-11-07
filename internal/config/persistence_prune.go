package config

import (
	"fmt"
	"strings"

	burnttoml "github.com/BurntSushi/toml"
)

// DeleteKeyPruningEmptyTable removes path and then removes its parent table when the removal
// left that table with no keys and no nested tables.
// The key's whole line is removed, so a set-then-delete round trip restores the original bytes.
func (e *OverlayEditor) DeleteKeyPruningEmptyTable(path []string) error {
	cleanPath, err := normalizeMutationPath(path)
	if err != nil {
		return err
	}
	document, err := parseOverlayDocument(e.content)
	if err != nil {
		return err
	}
	existing := document.findKeyValue(cleanPath)
	if existing == nil {
		return nil
	}
	start, end := lineReplacementOffsets(e.content, existing.raw)
	e.content = replaceOffsets(e.content, start, end, nil)
	if len(cleanPath) < 2 {
		return nil
	}
	parent := cleanPath[:len(cleanPath)-1]
	var decoded map[string]any
	if _, err := burnttoml.Decode(string(e.content), &decoded); err != nil {
		return fmt.Errorf("config: decode overlay after deleting %q: %w", strings.Join(path, "."), err)
	}
	var node any = decoded
	for _, key := range parent {
		table, ok := node.(map[string]any)
		if !ok {
			return nil
		}
		if node, ok = table[key]; !ok {
			return nil
		}
	}
	if table, ok := node.(map[string]any); ok && len(table) == 0 {
		return e.Delete(parent)
	}
	return nil
}
