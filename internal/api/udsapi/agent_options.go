package udsapi

import "github.com/compozy/agh/internal/api/core"

// WithAgentCatalog injects the projected resource-backed agent catalog.
func WithAgentCatalog(catalog core.AgentCatalog) Option {
	return func(server *Server) {
		server.agentCatalog = catalog
	}
}

// WithAgentDefinitionSync injects the daemon convergence seam used after authored mutations.
func WithAgentDefinitionSync(syncer core.AgentDefinitionSync) Option {
	return func(server *Server) {
		server.agentSync = syncer
	}
}

// WithAgentHistoryPurgers injects scoped soul and heartbeat revision cleanup.
func WithAgentHistoryPurgers(soulPurger core.SoulHistoryPurger, heartbeatPurger core.HeartbeatHistoryPurger) Option {
	return func(server *Server) {
		server.soulHistoryPurger = soulPurger
		server.heartbeatPurger = heartbeatPurger
	}
}
