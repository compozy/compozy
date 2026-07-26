package udsapi

import core "github.com/compozy/agh/internal/api/core"

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
