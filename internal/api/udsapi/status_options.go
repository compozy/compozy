package udsapi

import "github.com/compozy/agh/internal/api/core"

// WithObserver injects the runtime observer.
func WithObserver(observer core.Observer) Option {
	return func(server *Server) {
		server.observer = observer
	}
}

// WithSchemaStreamStatusReader injects daemon-global migration status diagnostics.
func WithSchemaStreamStatusReader(reader core.SchemaStreamStatusReader) Option {
	return func(server *Server) {
		server.schemaStreams = reader
	}
}
