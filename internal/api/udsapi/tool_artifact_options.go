package udsapi

import toolspkg "github.com/compozy/compozy/internal/tools"

// WithToolArtifactStore injects retained oversized result storage.
func WithToolArtifactStore(store toolspkg.ToolArtifactStore) Option {
	return func(server *Server) {
		server.toolArtifacts = store
	}
}
