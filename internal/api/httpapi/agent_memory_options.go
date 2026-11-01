package httpapi

import "github.com/compozy/compozy/internal/agentmemory"

func WithAgentMemory(store *agentmemory.Store) Option {
	return func(server *Server) { server.agentMemory = store }
}
