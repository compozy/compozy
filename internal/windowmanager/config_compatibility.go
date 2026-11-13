package windowmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
)

// WarnRetiredConfigKey reports a boundary translation once per process until v0.8.0.
func WarnRetiredConfigKey(key string, translated bool) {
	WarnSurfaceDeprecation(context.Background(), nil, RetiredConfigDeprecation(key, translated), "config", nil)
}

// RetiredConfigDeprecation describes a config translation until v0.8.0.
func RetiredConfigDeprecation(key string, translated bool) Deprecation {
	replacement := "remove window_manager." + key
	if translated {
		replacement = `window_manager.new_window_policy = "tab"`
	}
	entry := SurfaceDeprecation("window_manager." + key)
	entry.Replacement = replacement
	entry.Message = RetiredConfigWarning(key, translated)
	return entry
}

// RetiredConfigWarning is the operator-facing warning for a v0.7 config shim.
func RetiredConfigWarning(key string, translated bool) string {
	if translated {
		return `warning: window_manager.new_window_policy = "floating" is deprecated; use "tab" before v0.8.0.`
	}
	return "warning: window_manager." + key + " is retired and ignored; remove it before v0.8.0."
}

// DecodeWorkspaceConfigJSON returns effective overrides and separate input deprecations until v0.8.0.
func DecodeWorkspaceConfigJSON(data []byte) (WorkspaceConfig, []Deprecation, error) {
	var deprecations []Deprecation
	translated, err := translateWorkspaceConfigJSON(data, func(key string, translated bool) {
		deprecations = append(deprecations, RetiredConfigDeprecation(key, translated))
	})
	if err != nil {
		return WorkspaceConfig{}, nil, err
	}
	var decoded WorkspaceConfig
	decoder := json.NewDecoder(bytes.NewReader(translated))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return WorkspaceConfig{}, nil, err
	}
	return decoded, deprecations, nil
}

func translateDocumentOverridesJSON(data []byte) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	raw, present := fields["overrides"]
	if !present {
		return data, nil
	}
	overrides, _, err := DecodeWorkspaceConfigJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("decode workspace overrides: %w", err)
	}
	fields["overrides"], err = json.Marshal(overrides)
	if err != nil {
		return nil, err
	}
	return json.Marshal(fields)
}

func translateWorkspaceConfigJSON(data []byte, warn func(string, bool)) ([]byte, error) {
	var fields map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&fields); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("workspace config has trailing data")
	}
	for _, name := range []string{"raise_on_focus", "drag_away_policy", "group_move_modifier"} {
		if _, present := fields[name]; present {
			delete(fields, name)
			warn(name, false)
		}
	}
	for _, table := range []struct {
		name string
		keys []string
	}{
		{"snap", []string{"edge_band", "corner_reach", "exit_slack", "repeat_ratios"}},
		{"bindings", []string{"top_center", "bottom_center"}},
	} {
		if raw, present := fields[table.name]; present {
			var values map[string]json.RawMessage
			if err := json.Unmarshal(raw, &values); err != nil || values == nil {
				delete(fields, table.name)
				warn(table.name, false)
				continue
			}
			for _, name := range table.keys {
				if _, present := values[name]; present {
					delete(values, name)
					warn(table.name+"."+name, false)
				}
			}
			if len(values) != 0 {
				return nil, fmt.Errorf("unknown window_manager.%s key: %w", table.name, ErrInvalidCommand)
			}
			delete(fields, table.name)
		}
	}
	if raw, present := fields["new_window_policy"]; present {
		var policy string
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		if policy == "floating" {
			fields["new_window_policy"] = json.RawMessage(`"tab"`)
			warn("new_window_policy", true)
		}
	}
	return json.Marshal(fields)
}
