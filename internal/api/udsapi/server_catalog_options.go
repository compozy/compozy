package udsapi

import core "github.com/compozy/compozy/internal/api/core"

// WithModelCatalogService injects the daemon-owned provider model catalog service.
func WithModelCatalogService(service core.ModelCatalogService) Option {
	return func(server *Server) {
		server.modelCatalog = service
	}
}

// WithMarketplaceCatalogService injects the daemon-owned curated marketplace service.
func WithMarketplaceCatalogService(service core.MarketplaceCatalogService) Option {
	return func(server *Server) {
		server.marketplaceCatalog = service
	}
}

func WithContextWindowResolver(resolver core.ContextWindowResolver) Option {
	return func(server *Server) { server.contextWindowResolver = resolver }
}

// WithProviderUsageService injects profile-scoped provider usage.
func WithProviderUsageService(service core.ProviderUsageService) Option {
	return func(server *Server) { server.providerUsage = service }
}
