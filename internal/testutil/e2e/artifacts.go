package e2e

import (
	"os"
	"path/filepath"

	"sync"
	"testing"

	aghcontract "github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/store"
)

// ArtifactKind identifies one stable E2E diagnostic surface.
type ArtifactKind string

const (
	ArtifactKindTranscript           ArtifactKind = "transcript"
	ArtifactKindEvents               ArtifactKind = "events"
	ArtifactKindTransportOutputs     ArtifactKind = "transport_outputs"
	ArtifactKindNetworkMessages      ArtifactKind = "network_messages"
	ArtifactKindNetworkThreads       ArtifactKind = "network_threads"
	ArtifactKindNetworkDirectRooms   ArtifactKind = "network_direct_rooms"
	ArtifactKindNetworkWork          ArtifactKind = "network_work"
	ArtifactKindNetworkAudit         ArtifactKind = "network_audit"
	ArtifactKindAutomationRuns       ArtifactKind = "automation_runs"
	ArtifactKindTasks                ArtifactKind = "tasks"
	ArtifactKindTaskRuns             ArtifactKind = "task_runs"
	ArtifactKindBridgeHealth         ArtifactKind = "bridge_health"
	ArtifactKindBridgeRoutes         ArtifactKind = "bridge_routes"
	ArtifactKindBridgeDeliveryState  ArtifactKind = "bridge_delivery_state"
	ArtifactKindBridgeSecretBindings ArtifactKind = "bridge_secret_bindings"
	ArtifactKindProviderCalls        ArtifactKind = "provider_calls"
	ArtifactKindToolHostDiagnostics  ArtifactKind = "tool_host_diagnostics"
	ArtifactKindCombinedFlow         ArtifactKind = "combined_flow"
	ArtifactKindSessionSandbox       ArtifactKind = "session_sandbox"
	ArtifactKindBrowserTrace         ArtifactKind = "browser_trace"
	ArtifactKindBrowserScreenshots   ArtifactKind = "browser_screenshots"
	ArtifactKindBrowserConsole       ArtifactKind = "browser_console"
	ArtifactKindBrowserNetwork       ArtifactKind = "browser_network"
)

type artifactSpec struct {
	relativePath string
	isDir        bool
}

type captureFileTarget struct {
	sourcePath string
	targetPath string
}

const defaultArtifactSlug = "run"

var artifactSpecs = map[ArtifactKind]artifactSpec{
	ArtifactKindTranscript:           {relativePath: "transcript.json"},
	ArtifactKindEvents:               {relativePath: "events.json"},
	ArtifactKindTransportOutputs:     {relativePath: string(ArtifactKindTransportOutputs), isDir: true},
	ArtifactKindNetworkMessages:      {relativePath: "network_messages.json"},
	ArtifactKindNetworkThreads:       {relativePath: "network_threads.json"},
	ArtifactKindNetworkDirectRooms:   {relativePath: "network_direct_rooms.json"},
	ArtifactKindNetworkWork:          {relativePath: "network_work.json"},
	ArtifactKindNetworkAudit:         {relativePath: "network_audit.json"},
	ArtifactKindAutomationRuns:       {relativePath: "automation_runs.json"},
	ArtifactKindTasks:                {relativePath: "tasks.json"},
	ArtifactKindTaskRuns:             {relativePath: "task_runs.json"},
	ArtifactKindBridgeHealth:         {relativePath: "bridge_health.json"},
	ArtifactKindBridgeRoutes:         {relativePath: "bridge_routes.json"},
	ArtifactKindBridgeDeliveryState:  {relativePath: "bridge_delivery_state.json"},
	ArtifactKindBridgeSecretBindings: {relativePath: "bridge_secret_bindings.json"},
	ArtifactKindProviderCalls:        {relativePath: "provider_calls.json"},
	ArtifactKindToolHostDiagnostics:  {relativePath: "tool_host_diagnostics.json"},
	ArtifactKindCombinedFlow:         {relativePath: "combined_flow.json"},
	ArtifactKindSessionSandbox:       {relativePath: "session_sandbox.json"},
	ArtifactKindBrowserTrace:         {relativePath: "browser_trace.zip"},
	ArtifactKindBrowserScreenshots:   {relativePath: "browser_screenshots", isDir: true},
	ArtifactKindBrowserConsole:       {relativePath: "browser_console.json"},
	ArtifactKindBrowserNetwork:       {relativePath: "browser_network.json"},
}

// ArtifactEntry records one captured diagnostic artifact.
type ArtifactEntry struct {
	Kind      ArtifactKind `json:"kind"`
	Path      string       `json:"path"`
	MediaType string       `json:"media_type,omitempty"`
}

// ArtifactManifest is the stable per-run artifact index.
type ArtifactManifest struct {
	Version   int             `json:"version"`
	Artifacts []ArtifactEntry `json:"artifacts"`
}

// RuntimeArtifactManifest captures the stable daemon-runtime surfaces later
// integration suites need for debugging and parity assertions.
type RuntimeArtifactManifest struct {
	Version              int                      `json:"version"`
	WorkspaceRoot        string                   `json:"workspace_root,omitempty"`
	Home                 RuntimeHomeArtifact      `json:"home"`
	Logs                 RuntimeLogArtifact       `json:"logs"`
	Runs                 RuntimeRunArtifact       `json:"runs"`
	Transport            RuntimeTransportArtifact `json:"transport"`
	ArtifactRootDir      string                   `json:"artifact_root_dir,omitempty"`
	ArtifactManifestPath string                   `json:"artifact_manifest_path,omitempty"`
	CapturedArtifacts    ArtifactManifest         `json:"captured_artifacts"`
}

// RuntimeHomeArtifact captures the isolated AGH home layout used by a harness.
type RuntimeHomeArtifact struct {
	HomeDir          string `json:"home_dir,omitempty"`
	ConfigFile       string `json:"config_file,omitempty"`
	DatabaseFile     string `json:"database_file,omitempty"`
	DaemonSocket     string `json:"daemon_socket,omitempty"`
	DaemonInfo       string `json:"daemon_info,omitempty"`
	LogsDir          string `json:"logs_dir,omitempty"`
	NetworkAuditFile string `json:"network_audit_file,omitempty"`
}

// RuntimeLogArtifact captures the daemon log surfaces retained by the harness.
type RuntimeLogArtifact struct {
	DaemonLogFile  string `json:"daemon_log_file,omitempty"`
	ProcessLogFile string `json:"process_log_file,omitempty"`
}

// RuntimeRunArtifact captures the stable run-root surfaces retained by the harness.
type RuntimeRunArtifact struct {
	RootDir     string   `json:"root_dir,omitempty"`
	Directories []string `json:"directories,omitempty"`
}

// RuntimeTransportArtifact captures the public transport metadata shared by the harness.
type RuntimeTransportArtifact struct {
	HTTPBaseURL string `json:"http_base_url,omitempty"`
	HTTPHost    string `json:"http_host,omitempty"`
	HTTPPort    int    `json:"http_port,omitempty"`
	UDSBaseURL  string `json:"uds_base_url,omitempty"`
	SocketPath  string `json:"socket_path,omitempty"`
	CLIBinary   string `json:"cli_binary,omitempty"`
	CLIWorkdir  string `json:"cli_workdir,omitempty"`
}

// TransportOutputArtifact records one CLI/HTTP/UDS result retained for later diagnostics.
type TransportOutputArtifact struct {
	Name       string   `json:"name,omitempty"`
	Transport  string   `json:"transport,omitempty"`
	Command    []string `json:"command,omitempty"`
	URL        string   `json:"url,omitempty"`
	Method     string   `json:"method,omitempty"`
	StatusCode int      `json:"status_code,omitempty"`
	Stdout     string   `json:"stdout,omitempty"`
	Stderr     string   `json:"stderr,omitempty"`
	Error      string   `json:"error,omitempty"`
	Payload    any      `json:"payload,omitempty"`
}

// SessionSandboxArtifact captures both the public session sandbox
// projection and the fuller persisted metadata stored on disk for one session.
type SessionSandboxArtifact struct {
	SessionID    string                             `json:"session_id"`
	SessionState string                             `json:"session_state,omitempty"`
	StopReason   store.StopReason                   `json:"stop_reason,omitempty"`
	StopDetail   string                             `json:"stop_detail,omitempty"`
	API          *aghcontract.SessionSandboxPayload `json:"api,omitempty"`
	Persisted    *store.SessionSandboxMeta          `json:"persisted,omitempty"`
}

// ToolHostOperationOutcome classifies one tool-host operation result.
type ToolHostOperationOutcome string

const (
	ToolHostOutcomeAllowed ToolHostOperationOutcome = "allowed"
	ToolHostOutcomeBlocked ToolHostOperationOutcome = "blocked"
)

// ToolHostOperationDiagnostic records one observed tool-host action.
type ToolHostOperationDiagnostic struct {
	Operation        string                   `json:"operation"`
	Path             string                   `json:"path,omitempty"`
	Outcome          ToolHostOperationOutcome `json:"outcome,omitempty"`
	Error            string                   `json:"error,omitempty"`
	SideEffectPath   string                   `json:"side_effect_path,omitempty"`
	SideEffectExists bool                     `json:"side_effect_exists,omitempty"`
}

// ToolHostDiagnosticsArtifact groups tool-host observations for one session.
type ToolHostDiagnosticsArtifact struct {
	SessionID  string                        `json:"session_id,omitempty"`
	Operations []ToolHostOperationDiagnostic `json:"operations,omitempty"`
}

// CombinedFlowArtifact records the cross-domain identifiers and side effects
// that make a multi-domain failure diagnosable from one retained run.
type CombinedFlowArtifact struct {
	Scenario          string   `json:"scenario"`
	SessionID         string   `json:"session_id,omitempty"`
	Channel           string   `json:"channel,omitempty"`
	AutomationRunID   string   `json:"automation_run_id,omitempty"`
	TriggerID         string   `json:"trigger_id,omitempty"`
	JobID             string   `json:"job_id,omitempty"`
	TaskID            string   `json:"task_id,omitempty"`
	TaskRunID         string   `json:"task_run_id,omitempty"`
	BridgeID          string   `json:"bridge_id,omitempty"`
	NetworkMessageIDs []string `json:"network_message_ids,omitempty"`
	SideEffectPaths   []string `json:"side_effect_paths,omitempty"`
}

// Allowed returns the matching allowed operation diagnostic when present.
func (a ToolHostDiagnosticsArtifact) Allowed(operation string) (ToolHostOperationDiagnostic, bool) {
	return FindAllowedToolHostOperation(a.Operations, operation)
}

// Blocked returns the matching blocked operation diagnostic when present.
func (a ToolHostDiagnosticsArtifact) Blocked(operation string) (ToolHostOperationDiagnostic, bool) {
	return FindBlockedToolHostOperation(a.Operations, operation)
}

// FindAllowedToolHostOperation locates an allowed operation without relying on transcript text.
func FindAllowedToolHostOperation(
	operations []ToolHostOperationDiagnostic,
	operation string,
) (ToolHostOperationDiagnostic, bool) {
	return findToolHostOperation(operations, operation, ToolHostOutcomeAllowed, false)
}

// FindBlockedToolHostOperation locates a blocked operation without relying on transcript text.
func FindBlockedToolHostOperation(
	operations []ToolHostOperationDiagnostic,
	operation string,
) (ToolHostOperationDiagnostic, bool) {
	return findToolHostOperation(operations, operation, ToolHostOutcomeBlocked, true)
}

// ArtifactCollector captures and indexes stable E2E diagnostics.
type ArtifactCollector struct {
	rootDir      string
	manifestPath string

	mu      sync.Mutex
	entries map[ArtifactKind]ArtifactEntry
}

// NewArtifactCollector creates a per-test artifact directory. Passing tests clean it up,
// while failing tests keep it around for inspection.
func NewArtifactCollector(t testing.TB) *ArtifactCollector {
	t.Helper()

	dir, err := os.MkdirTemp("", "agh-e2e-"+sanitizePathComponent(t.Name())+"-")
	if err != nil {
		t.Fatalf("os.MkdirTemp(artifacts) error = %v", err)
	}

	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("retained E2E artifacts at %s", dir)
			return
		}
		_ = os.RemoveAll(dir)
	})

	return &ArtifactCollector{
		rootDir:      dir,
		manifestPath: filepath.Join(dir, "manifest.json"),
		entries:      make(map[ArtifactKind]ArtifactEntry),
	}
}
