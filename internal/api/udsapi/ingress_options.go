package udsapi

import "github.com/compozy/compozy/internal/api/core"

// WithIngressService installs the webhook ingress binding service.
func WithIngressService(service core.IngressService) Option {
	return func(server *Server) {
		server.ingress = service
	}
}

func WithMigrationNoticeStore(store core.MigrationNoticeStore) Option {
	return func(server *Server) { server.migrationNotices = store }
}
