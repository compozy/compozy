package config

import (
	_ "embed"
	"errors"
	"maps"
	"slices"
	"strings"
)

const (
	BuiltinCoordinatorAgentName = "coordinator"
)

// ErrAgentNameReserved marks attempts to author a catalog agent with a builtin identity.
var ErrAgentNameReserved = errors.New("config: agent name is reserved")

var (
	//go:embed prompts/coordinator.md
	coordinatorBuiltinPrompt string

	builtinAgentDefs = map[string]AgentDef{
		BuiltinCoordinatorAgentName: {
			Name:   BuiltinCoordinatorAgentName,
			Prompt: strings.TrimSpace(coordinatorBuiltinPrompt),
		},
	}
)

// BuiltinAgentNames returns the runtime-owned identity names.
func BuiltinAgentNames() []string {
	names := slices.AppendSeq(make([]string, 0, len(builtinAgentDefs)), maps.Keys(builtinAgentDefs))
	slices.Sort(names)
	return names
}

// BuiltinAgentDef returns a detached copy of a runtime-owned agent definition.
func BuiltinAgentDef(name string) (AgentDef, bool) {
	def, ok := builtinAgentDefs[normalizeBuiltinAgentName(name)]
	if !ok {
		return AgentDef{}, false
	}
	return CloneAgentDef(def), true
}

// IsReservedAgentName reports whether a catalog name collides with a builtin identity.
func IsReservedAgentName(name string) bool {
	return normalizeBuiltinAgentName(name) == BuiltinCoordinatorAgentName
}

func normalizeBuiltinAgentName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
