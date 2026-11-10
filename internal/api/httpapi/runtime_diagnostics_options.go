package httpapi

import (
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

// RouteTemplates returns the path templates of the mounted routes.
func (s *Server) RouteTemplates() []string {
	routes := s.engine.Routes()
	templates := make([]string, 0, len(routes))
	for _, route := range routes {
		templates = append(templates, route.Path)
	}
	return templates
}
