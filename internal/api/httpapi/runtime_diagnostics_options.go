package httpapi

import "github.com/compozy/compozy/internal/doctor"

// WithRuntimeMemorySnapshotSource injects daemon process-memory observations.
func WithRuntimeMemorySnapshotSource(source doctor.RuntimeMemorySnapshotSource) Option {
	return func(server *Server) { server.runtimeMemory = source }
}

// WithDeadEntitySource injects durable runtime-reliability diagnostics.
func WithDeadEntitySource(source doctor.DeadEntitySource) Option {
	return func(server *Server) { server.deadEntities = source }
}
