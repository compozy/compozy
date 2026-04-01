package providers

import (
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/core/provider/coderabbit"
)

func DefaultRegistry() *provider.Registry {
	registry := provider.NewRegistry()
	registry.Register(coderabbit.New())
	return registry
}
