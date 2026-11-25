package httpapi

import "github.com/compozy/compozy/internal/api/core"

func WithAccountService(service core.AccountService) Option {
	return func(server *Server) { server.account = service }
}
