package httpapi

import (
	"slices"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/doctor"
)

// WithRuntimeMemorySnapshotSource injects daemon process-memory observations.
func WithRuntimeMemorySnapshotSource(source doctor.RuntimeMemorySnapshotSource) Option {
	return func(server *Server) { server.runtimeMemory = source }
}

// WithHostLoadSnapshotSource injects host CPU pressure observations from load-aware admission.
func WithHostLoadSnapshotSource(source doctor.HostLoadSnapshotSource) Option {
	return func(server *Server) { server.hostLoad = source }
}

// WithDeadEntitySource injects durable runtime-reliability diagnostics.
func WithDeadEntitySource(source doctor.DeadEntitySource) Option {
	return func(server *Server) { server.deadEntities = source }
}

// WithTelemetryStatus injects OTLP export status for the status surface.
func WithTelemetryStatus(source core.TelemetryStatusSource) Option {
	return func(server *Server) { server.telemetryStatus = source }
}

// RouteTemplates returns a copy of the mounted templates captured before serving. Gin mutates its route tree
// on the first request, so reading Engine.Routes after Start would race with that request.
func (s *Server) RouteTemplates() []string {
	return slices.Clone(s.routeTemplates)
}
