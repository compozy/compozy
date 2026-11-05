package config

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
	"strings"

	burnttoml "github.com/BurntSushi/toml"
)

// Persisted-file migration adapted from #1297 by @rodrigo123antunes. Removal: v0.6.0.
const retiredAlwaysOnArchiveHeader = "\n# Archived [agents.stays_on]; values moved to [agents.always_on] " +
	"unless already set; these values are inactive.\n"

var retiredAgentKindTable = []string{AgentsDirName, retiredAgentKind}

// migrateRetiredAlwaysOnTable is the user-state migration for the v0.5.0 [agents.stays_on] table
// (#1239, SD-013). It moves each key into [agents.always_on] unless that table already sets it, as
// v0.5.1 resolved them, drops the old table, and appends the original values as an inactive
// comment archive. It returns the moved and archived key paths.
//
// The agents table is rendered as a unit and verified against the expected decoded values.
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
	rendered, err := renderAgentsTable(contents, source, want)
	if err != nil {
		return nil, nil, err
	}
	if !agentsTableEquals(rendered, want) {
		// Dotted keys may have no explicit table span for the editor to delete.
		values[AgentsDirName] = want
		var fallback bytes.Buffer
		if err := burnttoml.NewEncoder(&fallback).Encode(values); err != nil {
			return nil, nil, err
		}
		rendered = fallback.Bytes()
	}
	if !agentsTableEquals(rendered, want) {
		return nil, nil, fmt.Errorf("persisted agent settings migration changed values")
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
	result.WriteString("# Original config before agent-kind migration (inactive):\n")
	for line := range strings.SplitSeq(strings.TrimRight(string(contents), "\n"), "\n") {
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
