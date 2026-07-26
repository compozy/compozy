package udsapi

import (
	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/doctor"
	"github.com/compozy/agh/internal/memory"
)

// WithMemoryStore injects the memory store surfaced by the daemon.
func WithMemoryStore(store *memory.Store) Option {
	return func(server *Server) { server.memoryStore = store }
}

// WithDreamTrigger injects the dream-consolidation trigger surfaced by the daemon.
func WithDreamTrigger(trigger core.DreamTrigger) Option {
	return func(server *Server) { server.dreamTrigger = trigger }
}

// WithMemoryExtractorService injects the daemon-owned Memory v2 extractor runtime.
func WithMemoryExtractorService(service core.MemoryExtractorService) Option {
	return func(server *Server) { server.memoryExtractor = service }
}

// WithMemoryProviderService injects the daemon-owned MemoryProvider registry service.
func WithMemoryProviderService(service core.MemoryProviderService) Option {
	return func(server *Server) { server.memoryProviders = service }
}

// WithMemorySessionLedgerService injects the daemon-owned session ledger service.
func WithMemorySessionLedgerService(service core.MemorySessionLedgerService) Option {
	return func(server *Server) { server.memoryLedger = service }
}

// WithRuntimeMemorySnapshotSource injects daemon process-memory observations.
func WithRuntimeMemorySnapshotSource(source doctor.RuntimeMemorySnapshotSource) Option {
	return func(server *Server) { server.runtimeMemory = source }
}

// WithDeadEntitySource injects durable runtime-reliability diagnostics.
func WithDeadEntitySource(source doctor.DeadEntitySource) Option {
	return func(server *Server) { server.deadEntities = source }
}
