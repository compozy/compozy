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

// SetValueAtTarget writes value at path in target. Turning a provider back on
// (providers.<id>.enabled = true) in the global or profile layer deletes the key, and an emptied
// provider block, instead: an absent key means enabled (BR-1) and the toggle stays lossless
// (BR-11). Workspace layers keep the plain write so validation still rejects the key there.
func (e *OverlayEditor) SetValueAtTarget(target WriteTarget, path []string, value any) error {
	kind := target.Kind()
	if providerReEnable(path, value) && (kind == WriteTargetGlobalConfig || kind == WriteTargetProfileConfig) {
		return e.DeleteKeyPruningEmptyTable(path)
	}
	return e.SetValue(path, value)
}

// providerEnabledField is the providers.<id> key that turns a provider on or off.
const providerEnabledField = "enabled"

func providerReEnable(path []string, value any) bool {
	enabled, ok := value.(bool)
	return ok && enabled && len(path) == 3 &&
		strings.TrimSpace(path[0]) == "providers" && strings.TrimSpace(path[2]) == providerEnabledField
}
