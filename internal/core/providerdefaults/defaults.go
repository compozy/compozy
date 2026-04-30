package providerdefaults

import (
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/core/provider/coderabbit"
)

func DefaultRegistry() *provider.Registry {
	return DefaultRegistryForWorkspace("")
}

func DefaultRegistryForWorkspace(workspaceRoot string) *provider.Registry {
	registry := provider.NewRegistry()
	registry.Register(coderabbit.New(coderabbit.WithWorkingDir(workspaceRoot)))
	return registry
}
