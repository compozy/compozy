package config

import (
	"bytes"
	"fmt"
	"log/slog"

	"github.com/BurntSushi/toml"
	"github.com/goccy/go-yaml"
)

// Remove this boundary shim in v0.6.0 after the retired matcher migration window.
func ignoreRetiredAgentHookMatchers(data []byte) ([]byte, error) {
	var document map[string]any
	yamlFormat := yaml.Unmarshal(data, &document) == nil
	if !yamlFormat {
		if _, err := toml.Decode(string(data), &document); err != nil {
			return data, nil
		}
	}
	var entries []map[string]any
	switch hooks := document["hooks"].(type) {
	case []any:
		for _, hook := range hooks {
			if entry, ok := hook.(map[string]any); ok {
				entries = append(entries, entry)
			}
		}
	case []map[string]any:
		entries = hooks
	}
	var dropped []string
	for index, entry := range entries {
		matcher, ok := entry["matcher"].(map[string]any)
		if !ok {
			continue
		}
		for _, key := range []string{"compaction_reason", "compaction_strategy"} {
			if _, exists := matcher[key]; exists {
				delete(matcher, key)
				dropped = append(dropped, fmt.Sprintf("hooks[%d].matcher.%s", index, key))
			}
		}
	}
	if len(dropped) == 0 {
		return data, nil
	}
	var sanitized []byte
	if yamlFormat {
		var err error
		sanitized, err = yaml.Marshal(document)
		if err != nil {
			return nil, fmt.Errorf("encode agent frontmatter: %w", err)
		}
	} else {
		var buffer bytes.Buffer
		if err := toml.NewEncoder(&buffer).Encode(document); err != nil {
			return nil, fmt.Errorf("encode agent frontmatter: %w", err)
		}
		sanitized = buffer.Bytes()
	}
	slog.Warn("agent.retired_entries_ignored", "agent", document["name"], "entries", dropped)
	return sanitized, nil
}
