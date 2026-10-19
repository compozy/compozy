package udsapi

import "github.com/compozy/compozy/internal/api/core"

// WithSessionFileSearcher injects the session file search index.
func WithSessionFileSearcher(searcher core.SessionFileSearcher) Option {
	return func(server *Server) {
		server.sessionFiles = searcher
	}
}
