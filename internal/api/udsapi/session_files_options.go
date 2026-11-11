package udsapi

import "github.com/compozy/compozy/internal/api/core"

// WithSessionFileSearcher injects the session file search index.
func WithSessionFileSearcher(searcher core.SessionFileSearcher) Option {
	return func(server *Server) {
		server.sessionFiles = searcher
	}
}

func WithFileTreeReader(reader core.FileTreeReader) Option {
	return func(server *Server) { server.fileTree = reader }
}

func WithSessionChangesReader(reader core.SessionChangesReader) Option {
	return func(server *Server) { server.sessionChanges = reader }
}
