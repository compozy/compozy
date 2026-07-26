package config

import (
	_ "embed"
	"errors"
	"sort"
	"strings"
)

const (
	BuiltinCoordinatorAgentName     = "coordinator"
	BuiltinDreamingCuratorAgentName = "dreaming-curator"
)

// ErrAgentNameReserved marks attempts to author a catalog agent with a builtin identity.
var ErrAgentNameReserved = errors.New("config: agent name is reserved")

var (
	//go:embed prompts/coordinator.md
	coordinatorBuiltinPrompt string
	//go:embed prompts/dreaming-curator.md
	dreamingCuratorBuiltinPrompt string

	builtinAgentDefs = map[string]AgentDef{
		BuiltinCoordinatorAgentName: {
			Name:   BuiltinCoordinatorAgentName,
			Prompt: strings.TrimSpace(coordinatorBuiltinPrompt),
		},
		BuiltinDreamingCuratorAgentName: {
			Name:   BuiltinDreamingCuratorAgentName,
			Prompt: strings.TrimSpace(dreamingCuratorBuiltinPrompt),
		},
	}
)

// BuiltinAgentNames returns the reserved runtime-owned identity names.
func BuiltinAgentNames() []string {
	names := make([]string, 0, len(builtinAgentDefs))
	for name := range builtinAgentDefs {
		names = append(names, name)
	}
	sort.Strings(names)
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
	_, ok := builtinAgentDefs[normalizeBuiltinAgentName(name)]
	return ok
}

func normalizeBuiltinAgentName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
