package httpapi

import attachmentspkg "github.com/compozy/compozy/internal/attachments"

// WithSessionAttachmentStore injects durable session attachment storage.
func WithSessionAttachmentStore(store attachmentspkg.Store) Option {
	return func(server *Server) {
		server.sessionAttachments = store
	}
}
