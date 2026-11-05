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
//
// The common header-table layout is edited in place so comments elsewhere survive. When a table is
// written inline, the in-place edit cannot express the change, so the whole [agents] table is
// rendered again from its decoded values. Either result is accepted only when [agents] decodes to
// exactly the expected values.
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
	current := map[string]any{}
	if value, set := agents[AgentKindAlwaysOn]; set {
		currentTable, isCurrentTable := value.(map[string]any)
		if !isCurrentTable {
			return nil, nil, fmt.Errorf("%s.%s must be a table", AgentsDirName, AgentKindAlwaysOn)
		}
		current = currentTable
	}
	want := migratedAgentsTable(agents, table, current)
	rendered, err := moveRetiredAlwaysOnKeys(contents, source, table, current)
	if err != nil || !agentsTableEquals(rendered, want) {
		rendered, err = renderAgentsTable(contents, source, want)
		if err != nil {
			return nil, nil, fmt.Errorf("migrate %s.%s: %w", AgentsDirName, retiredAgentKind, err)
		}
		if !agentsTableEquals(rendered, want) {
			return nil, nil, fmt.Errorf(
				"migrate %s.%s: the rewritten [%s] table does not match its values",
				AgentsDirName, retiredAgentKind, AgentsDirName,
			)
		}
	}
	if len(table) == 0 {
		return rendered, []string{strings.Join(retiredAgentKindTable, ".")}, nil
	}
	names := make([]string, 0, len(table))
	for _, key := range slices.Sorted(maps.Keys(table)) {
		names = append(names, strings.Join(append(slices.Clone(retiredAgentKindTable), key), "."))
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

// migratedAgentsTable returns the [agents] values after the move: no old table, and always_on
// holding its own keys plus the old keys it did not set.
func migratedAgentsTable(agents, retired, current map[string]any) map[string]any {
	want := maps.Clone(agents)
	delete(want, retiredAgentKind)
	merged := maps.Clone(current)
	for key, value := range retired {
		if _, set := merged[key]; !set {
			merged[key] = value
		}
	}
	if _, had := agents[AgentKindAlwaysOn]; had || len(merged) > 0 {
		want[AgentKindAlwaysOn] = merged
	}
	return want
}

// moveRetiredAlwaysOnKeys edits the header-table layout in place, keeping unrelated bytes.
func moveRetiredAlwaysOnKeys(contents []byte, source string, retired, current map[string]any) ([]byte, error) {
	editor, err := newOverlayEditor(source, contents)
	if err != nil {
		return nil, err
	}
	keys := slices.Sorted(maps.Keys(retired))
	for _, key := range keys {
		if err := editor.Delete(append(slices.Clone(retiredAgentKindTable), key)); err != nil {
			return nil, err
		}
	}
	if err := editor.Delete(retiredAgentKindTable); err != nil {
		return nil, err
	}
	for _, key := range keys {
		if _, set := current[key]; set {
			continue
		}
		if err := editor.SetValue(append(slices.Clone(alwaysOnTable), key), retired[key]); err != nil {
			return nil, err
		}
	}
	return editor.Bytes()
}

// renderAgentsTable replaces every form of [agents] (header tables, dotted keys, or an inline
// value) with a table rendered from want. Comments inside the old [agents] table are not kept.
func renderAgentsTable(contents []byte, source string, want map[string]any) ([]byte, error) {
	editor, err := newOverlayEditor(source, contents)
	if err != nil {
		return nil, err
	}
	if err := editor.Delete([]string{AgentsDirName}); err != nil {
		return nil, err
	}
	if len(want) > 0 {
		if err := editor.SetTable([]string{AgentsDirName}, want); err != nil {
			return nil, err
		}
	}
	return editor.Bytes()
}

// agentsTableEquals reports whether rendered decodes to an [agents] table equal to want. Both sides
// are compared as canonical TOML, which sorts keys.
func agentsTableEquals(rendered []byte, want map[string]any) bool {
	var values map[string]any
	if _, err := burnttoml.Decode(string(rendered), &values); err != nil {
		return false
	}
	got, isTable := values[AgentsDirName].(map[string]any)
	if !isTable {
		got = map[string]any{}
	}
	var left, right bytes.Buffer
	if burnttoml.NewEncoder(&left).Encode(map[string]any{AgentsDirName: got}) != nil ||
		burnttoml.NewEncoder(&right).Encode(map[string]any{AgentsDirName: want}) != nil {
		return false
	}
	return left.String() == right.String()
}
