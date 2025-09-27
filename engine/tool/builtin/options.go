package builtin

import (
	"strings"

	"github.com/compozy/compozy/pkg/config"
)

// Options customize builtin registration and configuration overrides.
type Options struct {
	Definitions       []BuiltinDefinition
	EnableOverride    *bool
	ExtraExecCommands []config.NativeExecCommandConfig
}

func (o Options) definitions() []BuiltinDefinition {
	return append([]BuiltinDefinition(nil), o.Definitions...)
}

func (o Options) execAllowlistMerge(configured []config.NativeExecCommandConfig) []config.NativeExecCommandConfig {
	combined := make([]config.NativeExecCommandConfig, 0, len(configured)+len(o.ExtraExecCommands))
	combined = append(combined, configured...)
	combined = append(combined, o.ExtraExecCommands...)
	seen := make(map[string]struct{}, len(combined))
	result := make([]config.NativeExecCommandConfig, 0, len(combined))
	for _, entry := range combined {
		path := strings.TrimSpace(entry.Path)
		if path == "" {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		result = append(result, entry)
	}
	return result
}
