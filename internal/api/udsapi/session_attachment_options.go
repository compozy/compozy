package udsapi

import "github.com/compozy/compozy/internal/api/core"

// WithSessionAttachmentStore injects durable session attachment storage.
func WithSessionAttachmentStore(store core.SessionAttachmentStore) Option {
	return func(server *Server) {
		server.sessionAttachments = store
	}
}
