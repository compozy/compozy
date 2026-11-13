package windowmanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
)

var deprecatedConfigSurfaces sync.Map

// WarnRetiredConfigKey reports a boundary translation once per process until v0.8.0.
func WarnRetiredConfigKey(key string, translated bool) {
	if _, warned := deprecatedConfigSurfaces.LoadOrStore(key, struct{}{}); warned {
		return
	}
	message := RetiredConfigWarning(key, translated)
	replacement := "remove window_manager." + key
	if translated {
		replacement = `window_manager.new_window_policy = "tab"`
	}
	slog.Warn(message, "event", "window_manager.deprecated_surface_used", "surface", "window_manager."+key,
		"replacement", replacement, "actor_kind", "config", "removal_release", "v0.8.0")
}

// RetiredConfigWarning is the operator-facing warning for a v0.7 config shim.
func RetiredConfigWarning(key string, translated bool) string {
	if translated {
		return `warning: window_manager.new_window_policy = "floating" is deprecated; use "tab" before v0.8.0.`
	}
	return "warning: window_manager." + key + " is retired and ignored; remove it before v0.8.0."
}

// DecodeWorkspaceConfigJSON translates retired overrides at input boundaries until v0.8.0.
func DecodeWorkspaceConfigJSON(data []byte) (WorkspaceConfig, error) {
	translated, err := translateWorkspaceConfigJSON(data, WarnRetiredConfigKey)
	if err != nil {
		return WorkspaceConfig{}, err
	}
	var decoded WorkspaceConfig
	decoder := json.NewDecoder(bytes.NewReader(translated))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return WorkspaceConfig{}, err
	}
	return decoded, nil
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
	overrides, err := DecodeWorkspaceConfigJSON(raw)
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
			if err := json.Unmarshal(raw, &values); err != nil {
				return nil, err
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
