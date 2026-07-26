package httpapi

import core "github.com/compozy/agh/internal/api/core"

// WithToolApprovalIssuer injects the local approval-token issuer.
func WithToolApprovalIssuer(issuer core.ToolApprovalIssuer) Option {
	return func(server *Server) {
		server.toolApprovals = issuer
	}
}

// WithToolApprovalGrantService injects durable native-tool approval management.
func WithToolApprovalGrantService(service core.ToolApprovalGrantService) Option {
	return func(server *Server) {
		server.approvalGrants = service
	}
}
