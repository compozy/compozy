package toolenvstate

import (
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/runtime/toolenv"
)

const envExtensionKey = appstate.ExtensionKey("toolenv.environment")

// Store persists the tool environment on the provided application state.
func Store(state *appstate.State, env toolenv.Environment) {
	if state == nil || env == nil {
		return
	}
	state.SetExtension(envExtensionKey, env)
}

// Load retrieves a previously stored tool environment from application state.
func Load(state *appstate.State) (toolenv.Environment, bool) {
	if state == nil {
		return nil, false
	}
	value, ok := state.Extension(envExtensionKey)
	if !ok {
		return nil, false
	}
	env, ok := value.(toolenv.Environment)
	if !ok {
		return nil, false
	}
	return env, true
}
