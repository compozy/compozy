package config

import (
	"errors"
	"fmt"
	"os"
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
		// An inline-table ancestor hides the key from the line editor; fail like SetValue instead
		// of reporting a deletion that left the value in place.
		return document.ensureNoScalarPrefix(cleanPath)
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
// (BR-11). A profile write keeps an explicit true when the global layer disables the provider, so
// the profile still overrides it (BR-2). Workspace layers keep the plain write so validation still
// rejects the key there.
func (e *OverlayEditor) SetValueAtTarget(homePaths HomePaths, target WriteTarget, path []string, value any) error {
	path = e.AuthoredProviderEnabledPath(path)
	if !providerReEnable(path, value) {
		return e.SetValue(path, value)
	}
	switch target.Kind() {
	case WriteTargetGlobalConfig:
		return e.DeleteKeyPruningEmptyTable(path)
	case WriteTargetProfileConfig:
		disabledBelow, err := globalProviderDisabled(homePaths, strings.TrimSpace(path[1]))
		if err != nil {
			return err
		}
		if !disabledBelow {
			return e.DeleteKeyPruningEmptyTable(path)
		}
	}
	return e.SetValue(path, value)
}

// globalProviderDisabled reports whether the global config file sets providers.<id>.enabled = false.
func globalProviderDisabled(homePaths HomePaths, providerID string) (bool, error) {
	content, err := os.ReadFile(homePaths.ConfigFile)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("config: read global config for provider %q: %w", providerID, err)
	}
	var decoded struct {
		Providers map[string]struct {
			Enabled *bool `toml:"enabled"`
		} `toml:"providers"`
	}
	if _, err := burnttoml.Decode(string(content), &decoded); err != nil {
		return false, fmt.Errorf("config: decode global config for provider %q: %w", providerID, err)
	}
	// The loader canonicalizes provider names, so [providers.OPENCODE] disables opencode too.
	target := CanonicalProviderName(providerID)
	for name, provider := range decoded.Providers {
		if CanonicalProviderName(name) == target && provider.Enabled != nil && !*provider.Enabled {
			return true, nil
		}
	}
	return false, nil
}

// AuthoredProviderEnabledPath rewrites providers.<id>.enabled to the provider table name the
// overlay already uses for that canonical provider, so [providers.OPENCODE] is edited in place
// instead of gaining a sibling [providers.opencode] the loader folds into the same provider.
// Any other path, or an overlay without a matching table, is returned unchanged.
func (e *OverlayEditor) AuthoredProviderEnabledPath(path []string) []string {
	if len(path) != 3 || strings.TrimSpace(path[0]) != "providers" ||
		strings.TrimSpace(path[2]) != providerEnabledField {
		return path
	}
	document, err := parseOverlayDocument(e.content)
	if err != nil {
		return path
	}
	target := CanonicalProviderName(path[1])
	for _, expr := range document.expressions {
		if len(expr.path) >= 2 && expr.path[0] == "providers" && expr.path[1] != path[1] &&
			CanonicalProviderName(expr.path[1]) == target {
			return []string{path[0], expr.path[1], path[2]}
		}
	}
	return path
}

// providerEnabledField is the providers.<id> key that turns a provider on or off.
const providerEnabledField = "enabled"

func providerReEnable(path []string, value any) bool {
	enabled, ok := value.(bool)
	return ok && enabled && len(path) == 3 &&
		strings.TrimSpace(path[0]) == "providers" && strings.TrimSpace(path[2]) == providerEnabledField
}
