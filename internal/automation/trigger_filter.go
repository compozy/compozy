package automation

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	triggerFilterKindKey        = "kind"
	triggerFilterScopeKey       = "scope"
	triggerFilterSourceKey      = "source"
	triggerFilterWorkspaceIDKey = "workspace_id"
)

func exactFilterMatch(filter map[string]string, envelope ActivationEnvelope) bool {
	for rawPath, rawWant := range filter {
		entry, ok := triggerFilterEntryFromPath(rawPath, rawWant)
		if !ok || !entry.matches(envelope) {
			return false
		}
	}
	return true
}

type triggerFilter struct {
	entries []triggerFilterEntry
}

func (f triggerFilter) matches(envelope ActivationEnvelope) bool {
	for _, entry := range f.entries {
		if !entry.matches(envelope) {
			return false
		}
	}
	return true
}

type triggerFilterField uint8

const (
	triggerFilterFieldKind triggerFilterField = iota + 1
	triggerFilterFieldScope
	triggerFilterFieldWorkspaceID
	triggerFilterFieldSource
	triggerFilterFieldData
)

type triggerFilterEntry struct {
	field    triggerFilterField
	dataPath string
	want     string
}

func compileTriggerFilter(filter map[string]string) triggerFilter {
	if len(filter) == 0 {
		return triggerFilter{}
	}
	entries := make([]triggerFilterEntry, 0, len(filter))
	for rawPath, rawWant := range filter {
		entry, ok := triggerFilterEntryFromPath(rawPath, rawWant)
		if ok {
			entries = append(entries, entry)
		}
	}
	return triggerFilter{entries: entries}
}

func triggerFilterEntryFromPath(path string, want string) (triggerFilterEntry, bool) {
	trimmedPath := strings.TrimSpace(path)
	entry := triggerFilterEntry{
		want: strings.TrimSpace(want),
	}
	switch trimmedPath {
	case triggerFilterKindKey:
		entry.field = triggerFilterFieldKind
		return entry, true
	case triggerFilterScopeKey:
		entry.field = triggerFilterFieldScope
		return entry, true
	case triggerFilterWorkspaceIDKey:
		entry.field = triggerFilterFieldWorkspaceID
		return entry, true
	case triggerFilterSourceKey:
		entry.field = triggerFilterFieldSource
		return entry, true
	}

	dataPath, ok := strings.CutPrefix(trimmedPath, "data.")
	if !ok || strings.TrimSpace(dataPath) == "" {
		return triggerFilterEntry{}, false
	}
	entry.field = triggerFilterFieldData
	entry.dataPath = dataPath
	return entry, true
}

func (e triggerFilterEntry) matches(envelope ActivationEnvelope) bool {
	got, ok := e.value(envelope)
	return ok && got == e.want
}

func (e triggerFilterEntry) value(envelope ActivationEnvelope) (string, bool) {
	switch e.field {
	case triggerFilterFieldKind:
		return strings.TrimSpace(envelope.Kind), true
	case triggerFilterFieldScope:
		return string(envelope.Scope), true
	case triggerFilterFieldWorkspaceID:
		return strings.TrimSpace(envelope.WorkspaceID), true
	case triggerFilterFieldSource:
		return string(envelope.Source), true
	case triggerFilterFieldData:
		value, ok := lookupEnvelopeDataPath(envelope.Data, e.dataPath)
		if !ok {
			return "", false
		}
		return stringifyEnvelopeValue(value)
	default:
		return "", false
	}
}

func lookupEnvelopeDataPath(data map[string]any, path string) (any, bool) {
	var current any = data
	remaining := path
	for {
		segment, rest, found := strings.Cut(remaining, ".")
		key := strings.TrimSpace(segment)
		if key == "" {
			return nil, false
		}

		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[key]
			if !ok {
				return nil, false
			}
			current = next
		case map[string]string:
			next, ok := typed[key]
			if !ok {
				return nil, false
			}
			current = next
		default:
			return nil, false
		}

		if !found {
			return current, true
		}
		remaining = rest
	}
}

func cloneTriggerFilter(src triggerFilter) triggerFilter {
	if len(src.entries) == 0 {
		return triggerFilter{}
	}
	entries := make([]triggerFilterEntry, len(src.entries))
	copy(entries, src.entries)
	return triggerFilter{entries: entries}
}

func envelopeFilterValue(envelope ActivationEnvelope, path string) (string, bool) {
	entry, ok := triggerFilterEntryFromPath(path, "")
	if !ok {
		return "", false
	}
	return entry.value(envelope)
}

func stringifyEnvelopeValue(value any) (string, bool) {
	switch typed := value.(type) {
	case nil:
		return "", false
	case string:
		return typed, true
	case []byte:
		return string(typed), true
	case bool:
		return strconv.FormatBool(typed), true
	case int:
		return strconv.Itoa(typed), true
	case int8:
		return strconv.FormatInt(int64(typed), 10), true
	case int16:
		return strconv.FormatInt(int64(typed), 10), true
	case int32:
		return strconv.FormatInt(int64(typed), 10), true
	case int64:
		return strconv.FormatInt(typed, 10), true
	case uint:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint8:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint16:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint64:
		return strconv.FormatUint(typed, 10), true
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32), true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case time.Time:
		return typed.UTC().Format(time.RFC3339Nano), true
	case fmt.Stringer:
		return typed.String(), true
	default:
		return "", false
	}
}
