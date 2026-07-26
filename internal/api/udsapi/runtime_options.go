package udsapi

import (
	"log/slog"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/core"
	aghconfig "github.com/compozy/agh/internal/config"
)

// WithHomePaths overrides the resolved AGH home layout.
func WithHomePaths(homePaths aghconfig.HomePaths) Option {
	return func(server *Server) {
		server.homePaths = homePaths
		if !server.configSet {
			server.config = aghconfig.DefaultWithHome(homePaths)
		}
	}
}

// WithConfig overrides the runtime configuration used by the server.
func WithConfig(cfg *aghconfig.Config) Option {
	return func(server *Server) {
		if cfg != nil {
			server.config = *cfg
			server.configSet = true
		}
	}
}

// WithSocketPath overrides the Unix socket path served by the API.
func WithSocketPath(path string) Option {
	return func(server *Server) {
		server.socketPath = strings.TrimSpace(path)
	}
}

// WithLogger injects the server logger.
func WithLogger(logger *slog.Logger) Option {
	return func(server *Server) {
		server.logger = logger
	}
}

// WithStartedAt overrides the daemon start time reported by the API.
func WithStartedAt(startedAt time.Time) Option {
	return func(server *Server) { server.startedAt = startedAt }
}

// WithNow overrides the server clock, mainly for tests.
func WithNow(now func() time.Time) Option {
	return func(server *Server) { server.now = now }
}

// WithPollInterval overrides the SSE poll cadence.
func WithPollInterval(interval time.Duration) Option {
	return func(server *Server) { server.pollInterval = interval }
}

// WithSessionManager injects the runtime session manager.
func WithSessionManager(manager core.SessionManager) Option {
	return func(server *Server) { server.sessions = manager }
}

// WithSessionCatalog injects the daemon-owned session catalog.
func WithSessionCatalog(catalog core.SessionCatalog) Option {
	return func(server *Server) { server.sessionCatalog = catalog }
}

// WithTaskService injects the daemon-owned task service.
func WithTaskService(service core.TaskService) Option {
	return func(server *Server) { server.tasks = service }
}

// WithDaemonDrainController injects daemon-global admission control.
func WithDaemonDrainController(controller core.DaemonDrainController) Option {
	return func(server *Server) { server.drainController = controller }
}
