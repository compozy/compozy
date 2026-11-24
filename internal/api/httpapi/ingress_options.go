package httpapi

import "github.com/compozy/compozy/internal/api/core"

func WithIngressService(service core.IngressService) Option {
	return func(server *Server) { server.ingress = service }
}

func WithMigrationNoticeStore(store core.MigrationNoticeStore) Option {
	return func(server *Server) { server.migrationNotices = store }
}
