package compozy

import "time"

// ExecuteRequest is the unified request type for asynchronous executions.
type ExecuteRequest struct {
	Input   map[string]any `json:"input,omitempty"`
	Options map[string]any `json:"options,omitempty"`
}

// ExecuteSyncRequest is the unified request type for synchronous executions.
type ExecuteSyncRequest struct {
	Input   map[string]any `json:"input,omitempty"`
	Options map[string]any `json:"options,omitempty"`
	Timeout *time.Duration `json:"timeout,omitempty"`
}

// ExecuteResponse contains metadata about an asynchronous execution handle.
type ExecuteResponse struct {
	ExecID  string `json:"exec_id"`
	ExecURL string `json:"exec_url"`
}

// ExecuteSyncResponse represents the outcome of a synchronous execution.
type ExecuteSyncResponse struct {
	ExecID string         `json:"exec_id"`
	Output map[string]any `json:"output"`
}

// Mode declares the deployment strategy for the Compozy engine.
type Mode string

const (
	// ModeStandalone runs the engine with embedded dependencies.
	ModeStandalone Mode = "standalone"
	// ModeDistributed connects the engine to externally managed infrastructure.
	ModeDistributed Mode = "distributed"
)

// StandaloneTemporalConfig configures the embedded Temporal server when running in standalone mode.
type StandaloneTemporalConfig struct {
	DatabaseFile string
	FrontendPort int
	BindIP       string
	Namespace    string
	ClusterName  string
	EnableUI     bool
	UIPort       int
	LogLevel     string
	StartTimeout time.Duration
}

// StandaloneRedisConfig configures the embedded Redis server when running in standalone mode.
type StandaloneRedisConfig struct {
	Port             int
	Persistence      bool
	PersistenceDir   string
	SnapshotInterval time.Duration
	MaxMemory        int64
}

// ValidationError captures a validation failure discovered during reference checks.
type ValidationError struct {
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	Message      string `json:"message"`
}

// ValidationWarning captures a non-fatal validation warning.
type ValidationWarning struct {
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	Message      string `json:"message"`
}

// CircularDependency describes a detected dependency loop between resources.
type CircularDependency struct {
	Chain []string `json:"chain"`
}

// MissingReference captures an unresolved reference discovered during validation.
type MissingReference struct {
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	Reference    string `json:"reference"`
}

// ValidationReport contains the aggregate results of reference validation.
type ValidationReport struct {
	Valid           bool
	Errors          []ValidationError
	Warnings        []ValidationWarning
	ResourceCount   int
	CircularDeps    []CircularDependency
	MissingRefs     []MissingReference
	DependencyGraph map[string][]string
}
