package extensionpkg

import (
	"encoding/json"
	"fmt"
	"maps"
)

// UnmarshalTOML preserves JSON Schema objects inside a TOML tool table.
func (c *ToolConfig) UnmarshalTOML(value any) error {
	table, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("extension: tool config must be a TOML table")
	}
	copyTable := make(map[string]any, len(table))
	maps.Copy(copyTable, table)
	for _, key := range []string{manifestInputSchemaKey, manifestOutputSchemaKey} {
		text, ok := copyTable[key].(string)
		if !ok {
			continue
		}
		raw := json.RawMessage(text)
		if !json.Valid(raw) {
			return fmt.Errorf("extension: %s must contain valid JSON", key)
		}
		copyTable[key] = raw
	}
	encoded, err := json.Marshal(copyTable)
	if err != nil {
		return fmt.Errorf("extension: encode TOML tool config: %w", err)
	}
	type toolConfigAlias ToolConfig
	var decoded toolConfigAlias
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return fmt.Errorf("extension: decode TOML tool config: %w", err)
	}
	*c = ToolConfig(decoded)
	return nil
}
