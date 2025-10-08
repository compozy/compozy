package native

import (
	"sync"

	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/engine/tool/builtin/exec"
	"github.com/compozy/compozy/engine/tool/builtin/fetch"
	"github.com/compozy/compozy/engine/tool/builtin/filesystem"
)

// DefinitionProvider lazily constructs a builtin definition using the supplied environment.
type DefinitionProvider func(toolenv.Environment) builtin.BuiltinDefinition

var (
	providersOnce sync.Once
	providersMu   sync.RWMutex
	providers     []DefinitionProvider
	baseIndex     map[string]builtin.BuiltinDefinition
)

func Definitions(env toolenv.Environment) []builtin.BuiltinDefinition {
	providersOnce.Do(initProviders)
	providersMu.RLock()
	current := append([]DefinitionProvider(nil), providers...)
	providersMu.RUnlock()
	defs := make([]builtin.BuiltinDefinition, 0, len(current))
	for _, provider := range current {
		defs = append(defs, provider(env))
	}
	return defs
}

func DefinitionByID(id string) (builtin.BuiltinDefinition, bool) {
	providersOnce.Do(initProviders)
	providersMu.RLock()
	defer providersMu.RUnlock()
	def, ok := baseIndex[id]
	return def, ok
}

// RegisterProvider appends an additional definition provider. It should be
// invoked during init of environment-aware builtin packages.
func RegisterProvider(provider DefinitionProvider) {
	if provider == nil {
		return
	}
	providersOnce.Do(initProviders)
	providersMu.Lock()
	providers = append(providers, provider)
	providersMu.Unlock()
}

func initProviders() {
	baseIndex = make(map[string]builtin.BuiltinDefinition)
	addBaseDefinitions(filesystem.Definitions()...)
	addBaseDefinitions(exec.Definition())
	addBaseDefinitions(fetch.Definitions()...)
}

func addBaseDefinitions(defs ...builtin.BuiltinDefinition) {
	providersMu.Lock()
	defer providersMu.Unlock()
	if providers == nil {
		providers = make([]DefinitionProvider, 0, len(defs))
	}
	for _, def := range defs {
		defCopy := def
		providers = append(providers, func(toolenv.Environment) builtin.BuiltinDefinition {
			return defCopy
		})
		baseIndex[defCopy.ID] = defCopy
	}
}
