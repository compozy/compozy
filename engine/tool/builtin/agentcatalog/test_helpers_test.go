package agentcatalog

import (
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/runtime/toolenv/toolenvtest"
)

func newTestEnvironment(store resources.ResourceStore) toolenv.Environment {
	opts := []toolenvtest.Option{}
	if store != nil {
		opts = append(opts, toolenvtest.WithResourceStore(store))
	}
	return toolenvtest.NewNoopEnvironment(opts...)
}
