# Technical Specification: Temporal Standalone Mode

## Executive Summary

This specification defines the implementation of a standalone/embedded Temporal mode for Compozy using the official `go.temporal.io/server/temporal` package. Unlike the deprecated Temporalite approach, this solution uses the production-grade Temporal server code embedded as a library, configured with SQLite for zero-dependency local development while maintaining the same server capabilities used in production deployments.

**Key Technical Decisions:**
- Use `temporal.NewServer()` from `go.temporal.io/server/temporal` (official, non-deprecated)
- SQLite persistence with in-memory or file-based modes
- Configure all 4 Temporal services (frontend, history, matching, worker)
- Optional UI server for development debugging
- Add `Mode` configuration field supporting "remote" (default) and "standalone" modes
- **Dev/test focus**: Standalone optimized for development, but uses production server code

## System Architecture

### Domain Placement

**New Components:**
- `engine/worker/embedded/` - Embedded Temporal server wrapper and lifecycle management
  - `server.go` - Server creation and configuration
  - `config.go` - Configuration types and validation
  - `namespace.go` - Namespace initialization helper
- `pkg/config/config.go` - Extended `TemporalConfig` with mode selection and standalone options

**Modified Components:**
- `engine/infra/server/dependencies.go` - Server startup sequence to conditionally start embedded Temporal
- `pkg/config/definition/schema.go` - Registry entries for new configuration fields
- `pkg/config/provider.go` - Default values for standalone mode

**Unchanged Components:**
- `engine/worker/client.go` - Client creation remains unchanged (connects via `client.Dial()`)
- `engine/worker/mod.go` - Worker initialization unchanged
- All workflow/activity/task execution logic - zero impact

### Component Overview

**Embedded Server Manager (`engine/worker/embedded/server.go`):**
- Wraps `temporal.NewServer()` with opinionated configuration for local development
- Configures SQLite persistence (in-memory or file-based)
- Sets up all 4 services: frontend (7233), history (7234), matching (7235), worker (7236)
- Creates default namespace automatically
- Exposes `Start()`, `Stop()`, `FrontendAddress()` methods
- Handles graceful shutdown and resource cleanup

**UI Server Manager (Optional) (`engine/worker/embedded/ui.go`):**
- Wraps Temporal UI server for local debugging
- Connects to embedded Temporal frontend
- Exposes web UI on configurable port (default: 8233)
- Can be disabled via configuration

**Configuration Extension (`pkg/config/config.go`):**
- `TemporalConfig.Mode` - Mode selector ("remote" or "standalone")
- `TemporalConfig.Standalone` - Standalone-specific configuration
  - `DatabaseFile` - SQLite path or ":memory:" for ephemeral storage
  - `FrontendPort` - Frontend service port (default: 7233)
  - `EnableUI` - Enable web UI (default: true for dev)
  - `UIPort` - UI server port (default: 8233)
  - `LogLevel` - Server logging verbosity

**Server Lifecycle Integration (`engine/infra/server/dependencies.go`):**
- New function: `maybeStartStandaloneTemporal(cfg *config.Config)`
- Insertion point: BEFORE `maybeStartWorker()` in `setupDependencies()`
- Dynamically overrides `cfg.Temporal.HostPort` when standalone mode active
- Registers cleanup function for graceful server shutdown

## Implementation Design

### Core Interfaces

```go
// engine/worker/embedded/config.go
package embedded

import "time"

// Config holds embedded Temporal server configuration.
type Config struct {
	// DatabaseFile specifies SQLite database location.
	// Use ":memory:" for ephemeral in-memory storage.
	// Use file path for persistent storage across restarts.
	DatabaseFile string
	
	// FrontendPort is the gRPC port for the frontend service.
	// Default: 7233
	FrontendPort int
	
	// BindIP is the IP address to bind all services to.
	// Default: "127.0.0.1"
	BindIP string
	
	// Namespace is the default namespace to create on startup.
	// Default: "default"
	Namespace string
	
	// ClusterName is the Temporal cluster name.
	// Default: "compozy-standalone"
	ClusterName string
	
	// EnableUI enables the Temporal Web UI server.
	// Default: true
	EnableUI bool
	
	// UIPort is the HTTP port for the Web UI.
	// Default: 8233
	UIPort int
	
	// LogLevel controls server logging verbosity.
	// Values: "debug", "info", "warn", "error"
	// Default: "warn"
	LogLevel string
	
	// StartTimeout is the maximum time to wait for server startup.
	// Default: 30s
	StartTimeout time.Duration
}
```

```go
// engine/worker/embedded/server.go
package embedded

import (
	"context"
	"fmt"
	"net"
	"time"
	
	"go.temporal.io/server/common/config"
	"go.temporal.io/server/temporal"
)

// Server wraps an embedded Temporal server instance.
type Server struct {
	server       temporal.Server
	uiServer     *UIServer // nil if UI disabled
	config       *Config
	frontendAddr string
}

// NewServer creates but does not start an embedded Temporal server.
// Validates configuration and prepares all server components.
func NewServer(ctx context.Context, cfg *Config) (*Server, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	applyDefaults(cfg)
	temporalConfig, err := buildTemporalConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to build server config: %w", err)
	}
	// Create namespace in SQLite before server start
	if err := createNamespace(temporalConfig, cfg); err != nil {
		return nil, fmt.Errorf("failed to create namespace: %w", err)
	}
	server, err := temporal.NewServer(
		temporal.WithConfig(temporalConfig),
		temporal.ForServices(temporal.DefaultServices),
		temporal.WithStaticHosts(buildStaticHosts(cfg)),
		temporal.WithLogger(buildLogger(ctx, cfg)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporal server: %w", err)
	}
	s := &Server{
		server:       server,
		config:       cfg,
		frontendAddr: fmt.Sprintf("%s:%d", cfg.BindIP, cfg.FrontendPort),
	}
	if cfg.EnableUI {
		s.uiServer = newUIServer(cfg)
	}
	return s, nil
}

// Start starts the embedded Temporal server and optional UI server.
// Blocks until all services are ready or timeout occurs.
func (s *Server) Start(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.config.StartTimeout)
	defer cancel()
	if err := s.server.Start(); err != nil {
		return fmt.Errorf("failed to start temporal server: %w", err)
	}
	if err := s.waitForReady(ctx); err != nil {
		s.server.Stop()
		return fmt.Errorf("server startup timeout: %w", err)
	}
	if s.uiServer != nil {
		if err := s.uiServer.Start(ctx); err != nil {
			s.server.Stop()
			return fmt.Errorf("failed to start UI server: %w", err)
		}
	}
	return nil
}

// Stop gracefully shuts down the embedded server.
// Waits for in-flight operations to complete.
func (s *Server) Stop(ctx context.Context) error {
	if s.uiServer != nil {
		s.uiServer.Stop(ctx)
	}
	s.server.Stop()
	return nil
}

// FrontendAddress returns the gRPC address for Temporal clients.
// Format: "host:port" (e.g., "127.0.0.1:7233")
func (s *Server) FrontendAddress() string {
	return s.frontendAddr
}

// waitForReady polls the frontend service until ready or timeout.
func (s *Server) waitForReady(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", s.frontendAddr, 50*time.Millisecond)
			if err == nil {
				conn.Close()
				return nil
			}
		}
	}
}
```

```go
// engine/worker/embedded/builder.go
package embedded

import (
	"fmt"
	
	"github.com/google/uuid"
	"go.temporal.io/server/common/cluster"
	"go.temporal.io/server/common/config"
	"go.temporal.io/server/common/dynamicconfig"
	"go.temporal.io/server/common/membership/static"
	"go.temporal.io/server/common/metrics"
	sqliteplugin "go.temporal.io/server/common/persistence/sql/sqlplugin/sqlite"
	"go.temporal.io/server/common/primitives"
)

// buildTemporalConfig creates a temporal.Config from our simplified Config.
func buildTemporalConfig(cfg *Config) (*config.Config, error) {
	historyPort := cfg.FrontendPort + 1
	matchingPort := cfg.FrontendPort + 2
	workerPort := cfg.FrontendPort + 3
	metricsPort := cfg.UIPort + 1000
	
	connectAttrs := buildSQLiteConnectAttrs(cfg.DatabaseFile)
	
	return &config.Config{
		Global: config.Global{
			Metrics: &metrics.Config{
				Prometheus: &metrics.PrometheusConfig{
					ListenAddress: fmt.Sprintf("%s:%d", cfg.BindIP, metricsPort),
					HandlerPath:   "/metrics",
				},
			},
		},
		Persistence: config.Persistence{
			DefaultStore:     "sqlite-default",
			VisibilityStore:  "sqlite-default",
			NumHistoryShards: 1,
			DataStores: map[string]config.DataStore{
				"sqlite-default": {
					SQL: &config.SQL{
						PluginName:        sqliteplugin.PluginName,
						ConnectAttributes: connectAttrs,
						DatabaseName:      "temporal",
					},
				},
			},
		},
		ClusterMetadata: &cluster.Config{
			EnableGlobalNamespace:    false,
			FailoverVersionIncrement: 10,
			MasterClusterName:        cfg.ClusterName,
			CurrentClusterName:       cfg.ClusterName,
			ClusterInformation: map[string]cluster.ClusterInformation{
				cfg.ClusterName: {
					Enabled:                true,
					InitialFailoverVersion: 1,
					RPCAddress:             fmt.Sprintf("%s:%d", cfg.BindIP, cfg.FrontendPort),
					ClusterID:              uuid.NewString(),
				},
			},
		},
		DCRedirectionPolicy: config.DCRedirectionPolicy{
			Policy: "noop",
		},
		Services: map[string]config.Service{
			"frontend": {RPC: config.RPC{GRPCPort: cfg.FrontendPort, BindOnIP: cfg.BindIP}},
			"history":  {RPC: config.RPC{GRPCPort: historyPort, BindOnIP: cfg.BindIP}},
			"matching": {RPC: config.RPC{GRPCPort: matchingPort, BindOnIP: cfg.BindIP}},
			"worker":   {RPC: config.RPC{GRPCPort: workerPort, BindOnIP: cfg.BindIP}},
		},
		Archival: config.Archival{
			History:    config.HistoryArchival{State: "disabled"},
			Visibility: config.VisibilityArchival{State: "disabled"},
		},
		NamespaceDefaults: config.NamespaceDefaults{
			Archival: config.ArchivalNamespaceDefaults{
				History:    config.HistoryArchivalNamespaceDefaults{State: "disabled"},
				Visibility: config.VisibilityArchivalNamespaceDefaults{State: "disabled"},
			},
		},
		PublicClient: config.PublicClient{
			HostPort: fmt.Sprintf("%s:%d", cfg.BindIP, cfg.FrontendPort),
		},
	}, nil
}

// buildSQLiteConnectAttrs creates connection attributes for SQLite.
func buildSQLiteConnectAttrs(dbFile string) map[string]string {
	if dbFile == ":memory:" {
		return map[string]string{
			"mode":  "memory",
			"cache": "shared",
		}
	}
	return map[string]string{
		"_journal_mode": "WAL",
		"_synchronous":  "NORMAL",
	}
}

// buildStaticHosts creates static host configuration for all services.
func buildStaticHosts(cfg *Config) map[primitives.ServiceName]static.Hosts {
	return map[primitives.ServiceName]static.Hosts{
		primitives.FrontendService: static.SingleLocalHost(fmt.Sprintf("%s:%d", cfg.BindIP, cfg.FrontendPort)),
		primitives.HistoryService:  static.SingleLocalHost(fmt.Sprintf("%s:%d", cfg.BindIP, cfg.FrontendPort+1)),
		primitives.MatchingService: static.SingleLocalHost(fmt.Sprintf("%s:%d", cfg.BindIP, cfg.FrontendPort+2)),
		primitives.WorkerService:   static.SingleLocalHost(fmt.Sprintf("%s:%d", cfg.BindIP, cfg.FrontendPort+3)),
	}
}
```

### Data Models

**Configuration Extensions:**

```go
// pkg/config/config.go

type TemporalConfig struct {
	// Mode controls Temporal connection strategy.
	// Values: "remote" (default), "standalone"
	// Production MUST use "remote".
	Mode string `koanf:"mode" json:"mode" yaml:"mode" mapstructure:"mode" validate:"oneof=remote standalone" env:"TEMPORAL_MODE"`
	
	// HostPort specifies the Temporal server endpoint for remote mode.
	// Overridden automatically when Mode="standalone".
	HostPort string `koanf:"host_port" json:"host_port" yaml:"host_port" mapstructure:"host_port" env:"TEMPORAL_HOST_PORT"`
	
	Namespace string `koanf:"namespace" json:"namespace" yaml:"namespace" mapstructure:"namespace" env:"TEMPORAL_NAMESPACE"`
	TaskQueue string `koanf:"task_queue" json:"task_queue" yaml:"task_queue" mapstructure:"task_queue" env:"TEMPORAL_TASK_QUEUE"`
	
	// Standalone configures the embedded Temporal server.
	// Only applies when Mode="standalone".
	Standalone StandaloneConfig `koanf:"standalone" json:"standalone" yaml:"standalone" mapstructure:"standalone"`
}

type StandaloneConfig struct {
	// DatabaseFile is the SQLite database path.
	// Use ":memory:" for ephemeral in-memory storage (default for dev).
	// Use file path for persistent storage across restarts.
	DatabaseFile string `koanf:"database_file" json:"database_file" yaml:"database_file" mapstructure:"database_file" env:"TEMPORAL_STANDALONE_DATABASE_FILE"`
	
	// FrontendPort for Temporal frontend service.
	// Default: 7233
	// Other services use FrontendPort+1, +2, +3
	FrontendPort int `koanf:"frontend_port" json:"frontend_port" yaml:"frontend_port" mapstructure:"frontend_port" validate:"min=0,max=65535" env:"TEMPORAL_STANDALONE_FRONTEND_PORT"`
	
	// BindIP is the IP address to bind all services to.
	// Default: "127.0.0.1"
	BindIP string `koanf:"bind_ip" json:"bind_ip" yaml:"bind_ip" mapstructure:"bind_ip" env:"TEMPORAL_STANDALONE_BIND_IP"`
	
	// EnableUI enables the Temporal Web UI server.
	// Default: true (helpful for local debugging)
	EnableUI bool `koanf:"enable_ui" json:"enable_ui" yaml:"enable_ui" mapstructure:"enable_ui" env:"TEMPORAL_STANDALONE_ENABLE_UI"`
	
	// UIPort is the HTTP port for the Web UI.
	// Default: 8233
	UIPort int `koanf:"ui_port" json:"ui_port" yaml:"ui_port" mapstructure:"ui_port" validate:"min=0,max=65535" env:"TEMPORAL_STANDALONE_UI_PORT"`
	
	// LogLevel controls Temporal server logging verbosity.
	// Values: "debug", "info", "warn", "error"
	// Default: "warn"
	LogLevel string `koanf:"log_level" json:"log_level" yaml:"log_level" mapstructure:"log_level" validate:"oneof=debug info warn error" env:"TEMPORAL_STANDALONE_LOG_LEVEL"`
}
```

**Configuration Registry (`pkg/config/definition/schema.go`):**

```go
// Register temporal mode field
registry.Register(&FieldDef{
	Path:    "temporal.mode",
	Default: "remote",
	CLIFlag: "temporal-mode",
	EnvVar:  "TEMPORAL_MODE",
	Type:    reflect.TypeOf(""),
	Help:    "Temporal connection mode: remote (production) or standalone (dev/test only)",
})

// Register standalone config fields
registry.Register(&FieldDef{
	Path:    "temporal.standalone.database_file",
	Default: ":memory:",
	EnvVar:  "TEMPORAL_STANDALONE_DATABASE_FILE",
	Type:    reflect.TypeOf(""),
	Help:    "SQLite database path for standalone mode (:memory: or file path)",
})

registry.Register(&FieldDef{
	Path:    "temporal.standalone.frontend_port",
	Default: 7233,
	EnvVar:  "TEMPORAL_STANDALONE_FRONTEND_PORT",
	Type:    reflect.TypeOf(0),
	Help:    "Frontend service port for standalone mode",
})

registry.Register(&FieldDef{
	Path:    "temporal.standalone.bind_ip",
	Default: "127.0.0.1",
	EnvVar:  "TEMPORAL_STANDALONE_BIND_IP",
	Type:    reflect.TypeOf(""),
	Help:    "IP address to bind standalone Temporal services",
})

registry.Register(&FieldDef{
	Path:    "temporal.standalone.enable_ui",
	Default: true,
	EnvVar:  "TEMPORAL_STANDALONE_ENABLE_UI",
	Type:    reflect.TypeOf(true),
	Help:    "Enable Temporal Web UI in standalone mode",
})

registry.Register(&FieldDef{
	Path:    "temporal.standalone.ui_port",
	Default: 8233,
	EnvVar:  "TEMPORAL_STANDALONE_UI_PORT",
	Type:    reflect.TypeOf(0),
	Help:    "Web UI port for standalone mode",
})

registry.Register(&FieldDef{
	Path:    "temporal.standalone.log_level",
	Default: "warn",
	EnvVar:  "TEMPORAL_STANDALONE_LOG_LEVEL",
	Type:    reflect.TypeOf(""),
	Help:    "Temporal server log level (debug, info, warn, error)",
})
```

### API Endpoints

No new API endpoints required. This is a server-side infrastructure change only.

## Integration Points

### External Dependencies

**New Dependencies:**
1. `go.temporal.io/server` (latest stable version ~v1.25+)
   - License: MIT
   - Maintained by Temporal Technologies
   - Production-grade server code
   - ~100K+ lines, active development
   - Used in production by thousands of companies

2. `go.temporal.io/server/ui-server/v2` (optional, for Web UI)
   - License: MIT
   - Official Temporal Web UI server
   - Provides workflow debugging interface

3. `github.com/google/uuid` (transitive, likely already in project)
   - License: BSD-3-Clause
   - UUID generation for cluster IDs

**Dependency Justification:**
- **Why `go.temporal.io/server` instead of Temporalite:**
  - Temporalite is DEPRECATED (as of late 2023)
  - This is the official, production-grade Temporal server
  - Not a "testing-only" library - same code used in production
  - Active maintenance and development
  - No migration path needed - this IS the production path

- **Risk Assessment:**
  - Large dependency (~several MB), but necessary
  - Well-maintained by Temporal team
  - Security: Same security model as external Temporal
  - Performance: SQLite is lightweight for dev/test

## Impact Analysis

| Affected Component | Type of Impact | Description & Risk Level | Required Action |
|--------------------|----------------|--------------------------|-----------------|
| `pkg/config/config.go` | Schema Extension | Add Mode, Standalone fields. Non-breaking (new optional fields). Low risk. | Update struct, add validation |
| `pkg/config/definition/schema.go` | Registry Extension | Register new config fields. Low risk. | Add FieldDef entries |
| `pkg/config/provider.go` | Defaults Extension | Add standalone defaults. Low risk. | Add to defaults map |
| `engine/infra/server/dependencies.go` | Startup Sequence Modification | Insert embedded server startup before worker initialization. Medium risk (startup path). | Add conditional logic |
| `engine/worker/embedded/` (NEW) | New Package | Temporal server wrapper. No risk to existing code. | Create package |
| `go.mod` | Dependency Addition | Add `go.temporal.io/server`. Large dep (~several MB). Low risk. | `go get go.temporal.io/server` |
| `cli/cmd/start/start.go` | Documentation | Add CLI flag for `--temporal-mode`. Low risk. | Add flag, update help |
| `schemas/config.json` | Schema Extension | Extend Temporal schema with new fields. Low risk. | Add properties to JSON Schema |
| `docs/content/` | Documentation | Document standalone mode usage. Zero risk. | Create/update docs |

**Critical Path Dependencies:**
- No database schema changes
- No API contract changes
- Zero impact on workflow/activity execution logic
- Backward compatible: existing configurations continue to work (Mode defaults to "remote")

## Testing Approach

### Unit Tests

**`engine/worker/embedded/server_test.go`:**
- Should create server with valid config
- Should start and stop server successfully
- Should reject invalid config (bad port, bad log level, bad database path)
- Should return valid FrontendAddress after start
- Should create default namespace on startup
- Should handle start errors gracefully (port in use)
- Should handle stop errors gracefully
- Should timeout if server doesn't start within StartTimeout

**`engine/worker/embedded/config_test.go`:**
- Should validate Config fields
- Should apply defaults correctly
- Should build SQLite connect attributes for memory mode
- Should build SQLite connect attributes for file mode
- Should create valid Temporal config structure

**`pkg/config/config_test.go`:**
- Should validate Mode field (only "remote" or "standalone")
- Should validate Standalone.FrontendPort range
- Should validate Standalone.UIPort range
- Should validate Standalone.LogLevel values
- Should provide defaults for Standalone fields
- Should handle missing Standalone config when Mode="standalone"

### Integration Tests

**`test/integration/temporal/standalone_test.go`:**
- Should start Compozy server in standalone mode with memory persistence
- Should start Compozy server in standalone mode with file persistence
- Should execute simple workflow end-to-end
- Should persist workflows across server restarts (file-based SQLite)
- Should isolate namespaces correctly
- Should handle worker registration
- Should shut down cleanly without errors
- Should expose Web UI when EnableUI=true
- Should not expose Web UI when EnableUI=false

**`test/integration/temporal/mode_switching_test.go`:**
- Should start in remote mode (default)
- Should start in standalone mode when configured
- Should fail gracefully if embedded server fails to start
- Should fail gracefully if port is already in use
- Should log appropriate warnings about standalone limitations
- Should override HostPort when standalone mode active

**`test/integration/temporal/startup_lifecycle_test.go`:**
- Should start embedded server before worker
- Should wait for server readiness before proceeding
- Should clean up embedded server on shutdown
- Should handle server startup timeout
- Should handle concurrent startup/shutdown

### Test Data Requirements

**Fixtures (`test/integration/temporal/testdata/`):**
- `compozy-standalone-memory.yaml` - Config with in-memory mode
- `compozy-standalone-file.yaml` - Config with file-based persistence
- `compozy-standalone-no-ui.yaml` - Config with UI disabled
- `compozy-remote.yaml` - Remote mode config (existing)
- `simple-workflow.yaml` - Minimal workflow for testing

## Development Sequencing

### Build Order

1. **Embedded Server Package (First)**
   - Create `engine/worker/embedded/` package structure
   - Implement `Config` struct with validation
   - Implement `Server` struct with lifecycle management
   - Implement helper functions (`buildTemporalConfig`, `buildStaticHosts`, etc.)
   - Implement namespace creation helper
   - Add unit tests for server lifecycle
   - **Why first:** Core functionality; can be developed and tested independently

2. **Configuration Foundation (Second)**
   - Update `pkg/config/config.go` with Mode and Standalone fields
   - Add registry entries in `pkg/config/definition/schema.go`
   - Add defaults in `pkg/config/provider.go`
   - Add config validation tests
   - **Dependencies:** Requires (1) for embedded Config types

3. **Server Lifecycle Integration (Third)**
   - Modify `engine/infra/server/dependencies.go`
   - Add `maybeStartStandaloneTemporal()` function
   - Wire cleanup into existing cleanup chain
   - Add integration tests for startup/shutdown
   - **Dependencies:** Requires (1) and (2)

4. **Optional UI Server (Fourth)**
   - Implement `engine/worker/embedded/ui.go`
   - Add UI server lifecycle management
   - Add tests for UI server
   - **Dependencies:** Requires (1); can be done in parallel with (3)

5. **CLI and Documentation (Fifth)**
   - Add `--temporal-mode` CLI flag
   - Update help text and examples
   - Update JSON Schema
   - **Dependencies:** Requires (1), (2), (3) for full context

6. **Integration Tests and Examples (Sixth)**
   - Implement end-to-end tests with standalone mode
   - Test both memory and file-based persistence
   - Create example projects demonstrating standalone mode
   - **Dependencies:** Requires working implementation from (1)-(5)

### Technical Dependencies

**Blocking Dependencies:**
- Go 1.23+ (already satisfied)
- `go.temporal.io/server` compatible with project's Temporal SDK version
- SQLite support in host environment (universally available)

**Non-Blocking:**
- Documentation updates can proceed in parallel with implementation
- CLI flag additions can be done independently after config foundation

## Monitoring & Observability

### Metrics (via existing `infra/monitoring` package)

**New Metrics:**
```go
// Standalone mode status
compozy_temporal_standalone_enabled{mode="memory|file"} gauge
// Server lifecycle
compozy_temporal_standalone_starts_total counter
compozy_temporal_standalone_stops_total counter
compozy_temporal_standalone_errors_total{type="start|stop|namespace"} counter
// Server health
compozy_temporal_standalone_ready gauge
// Database stats (file mode only)
compozy_temporal_standalone_db_size_bytes gauge
// UI server status
compozy_temporal_standalone_ui_enabled gauge
```

### Logging (via existing `pkg/logger` package)

**Key Log Events:**
- `Info`: "Starting embedded Temporal server" (mode=standalone, database=:memory:|path, ui_enabled=true|false)
- `Info`: "Embedded Temporal server started successfully" (frontend_addr=..., ui_addr=..., duration=...)
- `Info`: "Created Temporal namespace" (namespace=..., cluster=...)
- `Warn`: "Standalone mode active - optimized for development, not production"
- `Error`: "Failed to start embedded Temporal server" (error=..., phase=config|namespace|server|ui)
- `Error`: "Embedded server startup timeout" (timeout=..., phase=...)
- `Info`: "Stopping embedded Temporal server"
- `Info`: "Embedded Temporal server stopped" (duration=...)
- `Debug`: "Embedded Temporal server lifecycle" (step=create|start|wait|ready|stop)
- `Debug`: "Temporal server configuration" (services=4, frontend_port=..., persistence=...)

### Grafana Dashboards

**Extend existing Temporal dashboard:**
- Add "Standalone Mode" section
- Display mode gauge (remote vs standalone)
- Show standalone-specific error rates
- Display startup/stop metrics
- Show database file size (file mode)
- Link to Temporal Web UI when in standalone mode (if enabled)

## Technical Considerations

### Key Decisions

**Decision 1: Use `temporal.NewServer()` vs Temporalite**
- **Rationale:** 
  - Temporalite is officially DEPRECATED (as of late 2023/early 2024)
  - `temporal.NewServer()` is the production-grade approach
  - Same server code used in production deployments
  - Active maintenance and long-term support
  - No migration path needed - this IS the production implementation
- **Trade-offs:** 
  - Larger dependency (~several MB vs Temporalite's lighter footprint)
  - More configuration complexity (but more control)
  - Requires understanding Temporal server architecture
- **Alternatives Rejected:**
  - Temporalite (deprecated, no future)
  - Docker-in-Docker (fragile, requires Docker, slow)
  - TestContainers (requires Docker, not suitable for dev mode)
  - `temporaltest` package (test-only, lacks persistence, missing features)

**Decision 2: SQLite Persistence Only**
- **Rationale:** 
  - Embedded server use case targets development/testing
  - SQLite is zero-dependency, built into Go
  - Supports both in-memory (fast, ephemeral) and file-based (persistent)
  - Same persistence backend that Temporal server supports
- **Trade-offs:**
  - Limited scalability (acceptable for dev/test)
  - Single-node only (acceptable for dev/test)
  - File-based mode requires disk I/O
- **Alternatives Rejected:**
  - Postgres/MySQL (too heavy for embedded use case)
  - In-memory only (lose persistence across restarts)

**Decision 3: Optional Web UI Server**
- **Rationale:**
  - Significantly improves debugging experience in local development
  - Temporal Web UI is the official debugging tool
  - Lightweight HTTP server, minimal overhead
  - Can be disabled if not needed
- **Trade-offs:**
  - Additional port required (default 8233)
  - Slight increase in startup time
  - More complexity in lifecycle management
- **Alternatives Rejected:**
  - Always enable UI (users may not need it, wastes resources)
  - Never enable UI (poor debugging experience)
  - External UI server (defeats purpose of standalone mode)

**Decision 4: Four-Service Architecture**
- **Rationale:**
  - Mirrors production Temporal deployment
  - Frontend (7233), History (7234), Matching (7235), Worker (7236)
  - Allows understanding production architecture in dev
  - Enables proper service isolation and testing
- **Trade-offs:**
  - Requires 4 ports (but all on localhost)
  - More complex than single-service
  - Slightly higher resource usage
- **Alternatives Rejected:**
  - Single-service (doesn't reflect production architecture)
  - Two-service (insufficient for realistic scenarios)

### Known Risks

**Risk 1: `go.temporal.io/server` Version Compatibility**
- **Description:** Temporal server package may have breaking changes between versions
- **Likelihood:** Low (Temporal maintains strong versioning discipline)
- **Impact:** High (could break embedded server on upgrade)
- **Mitigation:**
  - Pin exact version in `go.mod`
  - Test thoroughly before upgrading Temporal dependencies
  - Monitor Temporal release notes for breaking changes
  - Consider vendoring if stability critical

**Risk 2: Port Conflicts in Standalone Mode**
- **Description:** Ports 7233-7236 (and 8233 for UI) may be in use
- **Likelihood:** Medium (developers may have other services running)
- **Impact:** Medium (server fails to start, clear error message needed)
- **Mitigation:**
  - Provide clear error messages with resolution steps
  - Allow port configuration via config
  - Document common port conflicts
  - Integration tests for port conflict scenarios

**Risk 3: SQLite Database Corruption (File Mode)**
- **Description:** Improper shutdown or system crash may corrupt SQLite file
- **Likelihood:** Low (WAL mode reduces risk)
- **Impact:** Low to Medium (workflow state lost, must recreate)
- **Mitigation:**
  - Use WAL (Write-Ahead Logging) mode by default
  - Document backup procedures for important workflows
  - Recommend in-memory mode for disposable development
  - Provide clear error messages on corruption

**Risk 4: Standalone Mode Used in Production Accidentally**
- **Description:** Developer/operator sets `mode: standalone` in production config
- **Likelihood:** Medium (human error, copy/paste configs)
- **Impact:** Critical (single point of failure, data loss, performance issues)
- **Mitigation:**
  - Log PROMINENT WARNING on startup when standalone mode active
  - Check `RUNTIME_ENVIRONMENT` and fail if standalone + production
  - Document clearly in all examples and docs
  - Add validation that warns/fails in production-like environments

**Risk 5: Large `go.temporal.io/server` Dependency Size**
- **Description:** Temporal server package is large (~10-20MB compressed)
- **Likelihood:** Certain (inherent to approach)
- **Impact:** Low (longer initial download, larger binary)
- **Mitigation:**
  - Document dependency size in README
  - Binary size is acceptable trade-off for functionality
  - Consider lazy loading if becomes issue (future optimization)

### Special Requirements

**Performance:**
- Standalone mode startup must complete within 30 seconds (configurable via `StartTimeout`)
- Server readiness check must poll efficiently (100ms intervals)
- No performance degradation for remote mode (standalone code path only active when Mode="standalone")
- SQLite file size monitoring and logging
- Web UI must be responsive (<500ms page loads)

**Security:**
- Standalone mode uses localhost-only binding by default (BindIP: "127.0.0.1")
- No authentication required (acceptable for local development)
- SQLite file permissions: restrict to owner (0600) when using file-based persistence
- Disable archival (reduces attack surface)
- Log warnings if standalone mode configured with non-localhost BindIP

**Operability:**
- Clear error messages if server fails to start (port conflicts, permission issues)
- Graceful shutdown with timeout
- CLI must show standalone mode status in `compozy config show`
- Web UI link logged on startup when enabled
- Health checks for server readiness
- Prometheus metrics endpoint exposed

### Standards Compliance

This implementation follows:

- **architecture.mdc:** 
  - Clean separation between server lifecycle (infra) and client (worker)
  - Domain-driven structure with `engine/worker/embedded/` package
  - Infrastructure concerns properly isolated

- **go-coding-standards.mdc:**
  - Context propagation: `logger.FromContext(ctx)`, `config.FromContext(ctx)`
  - Error wrapping: `fmt.Errorf("...: %w", err)`
  - Function length: All functions <50 lines (split into helpers)
  - Constructor pattern: `NewServer(ctx, cfg)` with validation
  - Resource cleanup: Defer cleanup functions
  - No global state or singletons

- **global-config.mdc:** 
  - Configuration via registry with defaults, env vars, validation
  - Context-based config access
  - Precedence: defaults → YAML → env → CLI

- **backwards-compatibility.mdc:** 
  - No backwards compatibility required (project in alpha)
  - Greenfield approach - focus on best design

- **test-standards.mdc:** 
  - `t.Run("Should...")` pattern
  - Use `t.Context()` instead of `context.Background()`
  - Testify assertions
  - Integration tests in `test/integration/`
  - Table-driven tests for validation

- **logger-config.mdc:**
  - Context-first logging
  - `logger.FromContext(ctx)` pattern
  - No logger parameters or DI
  - Structured logging with key-value pairs

## Libraries Assessment

### Build vs Buy Decision

**Decision: BUY (Use `go.temporal.io/server`)**

**Primary Candidate: Temporal Server**
- **Repository:** https://github.com/temporalio/temporal
- **Package:** `go.temporal.io/server`
- **License:** MIT
- **Maintenance:** Active (Temporal Technologies official project)
- **Stars/Adoption:** 13K+ GitHub stars, thousands of production deployments
- **Integration Fit:** Perfect - official production server code embedded as library
- **Performance:** Production-grade, handles thousands of workflows/sec
- **Security:** Active security team, CVE monitoring, regular updates
- **Documentation:** Comprehensive official docs, active community
- **Pros:**
  - Official Temporal implementation - first-party support
  - NOT DEPRECATED (unlike Temporalite)
  - Production-grade code - same code used by large enterprises
  - Active development and maintenance
  - Comprehensive feature set (all Temporal capabilities)
  - Can scale from dev to production (same code path)
  - Well-documented configuration
  - Strong type safety and API design
- **Cons:**
  - Large dependency (~10-20MB)
  - Complex configuration (but provides full control)
  - Requires understanding Temporal architecture
  - Higher resource usage than lightweight alternatives (but acceptable for dev/test)

**Alternatives Considered:**

1. **Temporalite** (`github.com/temporalio/temporalite`)
   - **Status:** DEPRECATED (as of late 2023/early 2024)
   - **Pros:** Lightweight, simpler API
   - **Cons:** No future, deprecated, no maintenance, migration required
   - **Rejected:** Cannot use deprecated library for new feature

2. **`temporaltest` Package** (`go.temporal.io/sdk/testsuite`)
   - **Pros:** Built into SDK, lightweight
   - **Cons:** Test-only, no persistence, missing features (UI, metrics), not suitable for dev mode
   - **Rejected:** Too limited for development use case

3. **Build Custom Embedded Server**
   - **Pros:** Full control, no external dependency
   - **Cons:** Months of development, requires deep Temporal internals knowledge, high maintenance burden, security concerns
   - **Rejected:** Not feasible; would require 6-12 months of senior engineering time

4. **Docker-in-Docker**
   - **Pros:** Uses official Temporal Docker image
   - **Cons:** Requires Docker, slow startup (~10-30 seconds), fragile in CI, complex lifecycle
   - **Rejected:** Poor developer experience, defeats purpose of "standalone"

5. **TestContainers**
   - **Pros:** Well-known testing library
   - **Cons:** Requires Docker, test-only (not suitable for dev mode), slow startup
   - **Rejected:** Doesn't solve local development problem

**Final Recommendation:** Use `go.temporal.io/server/temporal.NewServer()`. It's the official, production-grade, non-deprecated solution. While the dependency is large, it provides the full Temporal feature set and is actively maintained by Temporal Technologies. The code is the same used in production deployments, ensuring a realistic development environment.

## Risk & Assumptions Registry

| Risk | Likelihood | Impact | Mitigation | Status |
|------|------------|--------|------------|--------|
| Server version compatibility breaks on upgrade | Low | High | Pin versions, test before upgrade | Open |
| Port conflicts prevent startup | Medium | Medium | Clear errors, configurable ports | Open |
| SQLite corruption (file mode) | Low | Medium | WAL mode, document backups | Open |
| Accidental production use | Medium | Critical | Validation checks, warnings, docs | Open |
| Large dependency size | Certain | Low | Document, acceptable trade-off | Accepted |
| Startup timeout in slow environments | Low | Low | Configurable timeout, efficient polling | Open |
| Web UI security exposure | Low | Low | Localhost-only by default | Open |

**Assumptions:**
1. Standalone mode is used exclusively for development and testing
2. Developers have SQLite support (universally available with Go)
3. Temporal SDK and server versions remain compatible (same organization maintains both)
4. Network ports 7233-7236 (and 8233 for UI) are available or configurable
5. File system write permissions available for SQLite file (when file-based persistence used)
6. Localhost binding is acceptable (BindIP can be configured if needed)
7. 30-second startup timeout is sufficient (configurable if needed)
8. Developers accept larger binary size for embedded server functionality

## Planning Artifacts (Must Be Generated)

This technical specification must be accompanied by:

1. **Documentation Plan** (`tasks/prd-temporal/_docs.md`)
   - User-facing documentation updates
   - Configuration reference
   - Quick start guides
   - Migration guides

2. **Examples Plan** (`tasks/prd-temporal/_examples.md`)
   - Example projects demonstrating standalone mode
   - Configuration templates
   - Common use cases

3. **Tests Plan** (`tasks/prd-temporal/_tests.md`)
   - Unit test coverage requirements
   - Integration test scenarios
   - Performance test criteria

## Next Steps

1. Review and approve this technical specification
2. Create implementation tasks from build order sequence
3. Set up development branch for Temporal standalone feature
4. Begin implementation with embedded server package (Step 1 of build order)
5. Iterate with tests at each stage to validate design decisions
6. Create PRD cleanup document if any technical content was misplaced in PRD

---

**Document Metadata:**
- **Author:** Technical Specification Agent
- **Date:** 2025-01-27 (Revised)
- **Status:** Draft - Awaiting Review
- **Approach:** `temporal.NewServer()` (Official, Non-Deprecated)
- **Reference:** https://github.com/abtinf/temporal-a-day/blob/main/001-all-in-one-hello/main.go
- **Dependencies:** tasks/prd-temporal/_prd.md (if exists)
- **Related Artifacts:**
  - tasks/prd-temporal/_docs.md (to be created)
  - tasks/prd-temporal/_examples.md (to be created)
  - tasks/prd-temporal/_tests.md (to be created)

## Appendix: Code References

**Based on:** https://github.com/abtinf/temporal-a-day/blob/main/001-all-in-one-hello/main.go

**Key Patterns from Reference:**
1. SQLite configuration with in-memory mode
2. Four-service architecture setup
3. Namespace creation via `sqliteschema.CreateNamespaces()`
4. UI server integration
5. Static host configuration
6. Prometheus metrics endpoint
