package httpapi

import toolspkg "github.com/compozy/agh/internal/tools"

// WithClarifyBroker injects the boot-scoped clarification authority.
func WithClarifyBroker(broker toolspkg.ClarifyBroker) Option {
	return func(server *Server) {
		server.clarify = broker
	}
}
