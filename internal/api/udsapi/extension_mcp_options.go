package udsapi

import mcppkg "github.com/compozy/compozy/internal/mcp"

// WithExtensionService injects daemon-backed extension management handlers.
func WithExtensionService(service ExtensionService) Option {
	return func(server *Server) {
		server.extensions = service
	}
}

// WithHostedMCP injects the hosted AGH MCP session exposure service.
func WithHostedMCP(service *mcppkg.HostedService) Option {
	return func(server *Server) {
		server.hostedMCP = service
	}
}

// WithMCPHostAPI injects the workspace-bound Host API façade used by compozy mcp serve.
func WithMCPHostAPI(service mcppkg.HostAPIInvoker) Option {
	return func(server *Server) {
		server.mcpHostAPI = service
	}
}
