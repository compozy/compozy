package config

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	burnttoml "github.com/BurntSushi/toml"
)

const retiredAlwaysOnArchiveHeader = "\n# Archived [agents.stays_on] table; v0.5.1 renamed it to [agents.always_on]. " +
	"Its keys moved there unless already set; these values are inactive.\n"

// retiredAgentKind is the v0.5.0 spelling of always_on. v0.5.1 accepted it with a warning and v0.5.2
// removed it (#1239). It stays named here only so each rejection can point at the replacement.
const retiredAgentKind = "stays_on"

// ErrConfigPathRemoved marks a config path whose table was renamed and no longer resolves.
var ErrConfigPathRemoved = errors.New("config: removed config path")

var (
	retiredAgentKindTable = []string{AgentsDirName, retiredAgentKind}
	alwaysOnTable         = []string{AgentsDirName, AgentKindAlwaysOn}
)

// ValidateAgentKind accepts on_demand, always_on, and empty (on_demand). Callers that compare a
// requested kind with the stored one validate it first, so a bad value is not reported as a change.
func ValidateAgentKind(kind string) error {
	switch (AgentDef{Kind: kind}).EffectiveKind() {
	case AgentKindOnDemand, AgentKindAlwaysOn:
		return nil
	case retiredAgentKind:
		return fmt.Errorf(
			"%w (%s was renamed to %s in v0.5.1)",
			ErrAgentKindInvalid,
			retiredAgentKind,
			AgentKindAlwaysOn,
		)
	default:
		return ErrAgentKindInvalid
	}
}

// RejectRemovedConfigPath reports a path under the renamed [agents.stays_on] table and names its
// [agents.always_on] replacement, so config get/set/unset and the config tools do not answer
// "not found" for a key that moved.
func RejectRemovedConfigPath(path []string) error {
	replacement, ok := alwaysOnReplacementPath(path)
	if !ok {
		return nil
	}
	return fmt.Errorf(
		"%w %q: use %q",
		ErrConfigPathRemoved,
		strings.Join(path, "."),
		strings.Join(replacement, "."),
	)
}

// RejectRemovedDottedConfigPath is RejectRemovedConfigPath for a dotted path.
func RejectRemovedDottedConfigPath(raw string) error {
	return RejectRemovedConfigPath(strings.Split(strings.TrimSpace(raw), "."))
}

func rejectRemovedAlwaysOnTableKeys(source string, keys []burnttoml.Key) error {
	for _, key := range sortedTOMLKeys(keys) {
		replacement, ok := alwaysOnReplacementPath(key)
		if !ok {
			continue
		}
		return fmt.Errorf(
			"removed config key %q in %q: use %q",
			key.String(),
			source,
			burnttoml.Key(replacement).String(),
		)
	}
	return nil
}

func alwaysOnReplacementPath(path []string) ([]string, bool) {
	if len(path) < len(retiredAgentKindTable) ||
		!slices.Equal(path[:len(retiredAgentKindTable)], retiredAgentKindTable) {
		return nil, false
	}
	return append(slices.Clone(alwaysOnTable), path[len(retiredAgentKindTable):]...), true
}

// migrateRetiredAlwaysOnTable is the user-state migration for the v0.5.0 [agents.stays_on] table
// (#1239, SD-013). It moves each key into [agents.always_on] unless that table already sets it, as
// v0.5.1 resolved them, drops the old table, and appends the original values as an inactive
// comment archive. It returns the moved and archived key paths.
func migrateRetiredAlwaysOnTable(contents []byte, source string) ([]byte, []string, error) {
	var values map[string]any
	if _, err := burnttoml.Decode(string(contents), &values); err != nil {
		return nil, nil, err
	}
	agents, isTable := values[AgentsDirName].(map[string]any)
	if !isTable {
		return contents, nil, nil
	}
	retired, exists := agents[retiredAgentKind]
	if !exists {
		return contents, nil, nil
	}
	table, ok := retired.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("%s.%s must be a table", AgentsDirName, retiredAgentKind)
	}
	current, hasCurrent := agents[AgentKindAlwaysOn].(map[string]any)
	if !hasCurrent {
		current = map[string]any{}
	}
	editor, err := newOverlayEditor(source, contents)
	if err != nil {
		return nil, nil, err
	}
	keys := slices.Sorted(maps.Keys(table))
	names := make([]string, 0, len(keys)+1)
	for _, key := range keys {
		if err := editor.Delete(append(slices.Clone(retiredAgentKindTable), key)); err != nil {
			return nil, nil, err
		}
		names = append(names, strings.Join(append(slices.Clone(retiredAgentKindTable), key), "."))
	}
	if err := editor.Delete(retiredAgentKindTable); err != nil {
		return nil, nil, err
	}
	for _, key := range keys {
		if _, set := current[key]; set {
			continue
		}
		if err := editor.SetValue(append(slices.Clone(alwaysOnTable), key), table[key]); err != nil {
			return nil, nil, err
		}
	}
	rendered, err := editor.Bytes()
	if err != nil {
		return nil, nil, err
	}
	if len(names) == 0 {
		return rendered, []string{strings.Join(retiredAgentKindTable, ".")}, nil
	}
	var archive bytes.Buffer
	if err := burnttoml.NewEncoder(&archive).
		Encode(map[string]any{AgentsDirName: map[string]any{retiredAgentKind: table}}); err != nil {
		return nil, nil, fmt.Errorf("archive retired %s.%s table: %w", AgentsDirName, retiredAgentKind, err)
	}
	result := bytes.NewBuffer(rendered)
	result.WriteString(retiredAlwaysOnArchiveHeader)
	for line := range strings.SplitSeq(strings.TrimRight(archive.String(), "\n"), "\n") {
		result.WriteString("# " + line + "\n")
	}
	return result.Bytes(), names, nil
}
