package udsapi

import (
	core "github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/store"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

// WithNetworkService injects the runtime network manager.
func WithNetworkService(service core.NetworkService) Option {
	return func(server *Server) {
		server.network = service
	}
}

// WithNetworkStore injects the persisted network query store.
func WithNetworkStore(store core.NetworkStore) Option {
	return func(server *Server) {
		server.networkStore = store
	}
}

// WithNetworkUsageStore injects the workspace network usage ledger reader.
func WithNetworkUsageStore(usage store.NetworkUsageStore) Option {
	return func(server *Server) {
		server.networkUsage = usage
	}
}

// WithCoordinationService injects the atomic coordination command boundary.
func WithCoordinationService(service workspacepkg.CoordinationCommands) Option {
	return func(server *Server) {
		server.coordination = service
	}
}
