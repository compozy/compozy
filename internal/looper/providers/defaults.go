package providers

import (
	"github.com/compozy/looper/internal/looper/provider"
	"github.com/compozy/looper/internal/looper/provider/coderabbit"
)

func DefaultRegistry() *provider.Registry {
	registry := provider.NewRegistry()
	registry.Register(coderabbit.New())
	return registry
}
