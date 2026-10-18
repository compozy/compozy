package httpapi

import "github.com/compozy/compozy/internal/api/core"

func WithAppTokenManager(manager core.AppTokenManager) Option {
	return func(server *Server) { server.appTokens = manager }
}

func WithOpenAPIDocument(document *core.OpenAPIDocument) Option {
	return func(server *Server) { server.openAPI = document }
}
