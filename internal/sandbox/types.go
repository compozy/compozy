// Package sandbox defines execution-sandbox contracts shared by
// daemon-native providers, session orchestration, and ACP launch plumbing.
package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
)

// Backend identifies the execution sandbox backend implementation.
type Backend string

const (
	// BackendLocal runs agents as local daemon-host subprocesses.
	BackendLocal Backend = "local"
	// BackendDaytona runs agents inside Daytona sandboxes.
	BackendDaytona Backend = "daytona"
	// BackendE2B is reserved for a future E2B provider.
	BackendE2B Backend = "e2b"
)

// ErrSandboxNotFound reports that a provider could not find a remote
// sandbox matching daemon-owned identity labels.
var ErrSandboxNotFound = errors.New("sandbox: remote sandbox not found")

// Valid reports whether b is a known backend identifier.
func (b Backend) Valid() bool {
	switch b {
	case BackendLocal, BackendDaytona, BackendE2B:
		return true
	default:
		return false
	}
}

// SyncMode controls workspace synchronization between local and runtime roots.
type SyncMode string

const (
	// SyncModeNone disables automatic workspace synchronization.
	SyncModeNone SyncMode = "none"
	// SyncModeSessionBidirectional syncs local-to-runtime on start and runtime-to-local on stop.
	SyncModeSessionBidirectional SyncMode = "session-bidirectional"
	// SyncModeTurnBidirectional is reserved for future turn-boundary synchronization.
	SyncModeTurnBidirectional SyncMode = "turn-bidirectional"
)

// Valid reports whether m is a known sync mode.
func (m SyncMode) Valid() bool {
	switch m {
	case SyncModeNone, SyncModeSessionBidirectional, SyncModeTurnBidirectional:
		return true
	default:
		return false
	}
}

// PersistenceMode controls whether provider instances are reused or discarded.
type PersistenceMode string

const (
	// PersistenceTransient destroys the runtime sandbox when the session stops.
	PersistenceTransient PersistenceMode = "transient"
	// PersistenceReuse keeps the runtime sandbox available for reuse.
	PersistenceReuse PersistenceMode = "reuse"
	// PersistenceArchive archives the runtime sandbox when possible.
	PersistenceArchive PersistenceMode = "archive"
)

// Valid reports whether m is a known persistence mode.
func (m PersistenceMode) Valid() bool {
	switch m {
	case PersistenceTransient, PersistenceReuse, PersistenceArchive:
		return true
	default:
		return false
	}
}

// DaytonaStartupSource identifies which Daytona startup input is authoritative.
type DaytonaStartupSource string

const (
	// DaytonaStartupSourceImage starts a sandbox from an image.
	DaytonaStartupSourceImage DaytonaStartupSource = "image"
	// DaytonaStartupSourceSnapshot starts a sandbox from a pre-baked snapshot.
	DaytonaStartupSourceSnapshot DaytonaStartupSource = "snapshot"
)

// NetworkPolicy is the resolved provider-neutral network intent.
type NetworkPolicy struct {
	AllowPublicIngress bool
	AllowOutbound      bool
	AllowList          []string
	DenyList           []string
	Required           bool
}

// DaytonaConfig is the resolved Daytona-specific provider policy.
type DaytonaConfig struct {
	APIURL        string
	Target        string
	Image         string
	Snapshot      string
	Class         string
	AutoStop      string
	AutoArchive   string
	StartupSource DaytonaStartupSource
	StartupRef    string
}

// Resolved is the workspace-selected sandbox profile after defaults and
// backend policy have been applied.
type Resolved struct {
	Profile        string
	Backend        Backend
	SyncMode       SyncMode
	Persistence    PersistenceMode
	RuntimeRootDir string
	DestroyOnStop  bool
	Env            map[string]string
	SecretEnv      map[string]string
	Network        NetworkPolicy
	Daytona        *DaytonaConfig
}

// SessionState is the provider runtime state persisted for a session.
type SessionState struct {
	SandboxID             string
	Backend               Backend
	Profile               string
	State                 string
	InstanceID            string
	RuntimeRootDir        string
	RuntimeAdditionalDirs []string
	ProviderState         json.RawMessage
	SSHAccessExpiresAt    *time.Time
	PreparedAt            time.Time
}

// PrepareRequest carries all daemon state needed to prepare a sandbox.
type PrepareRequest struct {
	SessionID           string
	WorkspaceID         string
	SandboxID           string
	InstanceID          string
	LocalRootDir        string
	LocalAdditionalDirs []string
	Sandbox             Resolved
	AgentCommand        string
	AgentEnv            []string
	Permissions         string
	ResumeACPState      string
	ProviderState       json.RawMessage
}

// FindSandboxRequest carries daemon identity for provider-side lookup of
// a partially-created remote sandbox.
type FindSandboxRequest struct {
	SessionID           string
	WorkspaceID         string
	SandboxID           string
	LocalRootDir        string
	LocalAdditionalDirs []string
	Sandbox             Resolved
	ProviderState       json.RawMessage
	Labels              map[string]string
}

// Prepared is the result of preparing an execution sandbox for a session.
type Prepared struct {
	State                 SessionState
	RuntimeRootDir        string
	RuntimeAdditionalDirs []string
	Launcher              Launcher
	Launch                LaunchSpec
	ToolHost              ToolHost
}

// SyncReason explains why a provider sync operation is running.
type SyncReason string

const (
	// SyncReasonStart syncs before launching the agent.
	SyncReasonStart SyncReason = "start"
	// SyncReasonTurn is reserved for future turn-boundary synchronization.
	SyncReasonTurn SyncReason = "turn"
	// SyncReasonStop syncs during normal session stop.
	SyncReasonStop SyncReason = "stop"
	// SyncReasonCrash syncs during crash recovery.
	SyncReasonCrash SyncReason = "crash"
)

// SyncDirection identifies the direction of a workspace synchronization.
type SyncDirection string

const (
	// SyncDirectionToRuntime syncs local workspace files into the runtime.
	SyncDirectionToRuntime SyncDirection = "to_runtime"
	// SyncDirectionFromRuntime syncs runtime workspace files back to local storage.
	SyncDirectionFromRuntime SyncDirection = "from_runtime"
)

// SyncOptions carries daemon decisions that affect one provider sync run.
type SyncOptions struct {
	Reason          SyncReason
	ExcludePatterns []string
}

// SyncResult reports provider-observed transfer statistics.
type SyncResult struct {
	FilesSynced      int
	BytesTransferred int64
	Errors           []string
}

// LaunchSpec describes the ACP-capable command to start inside a sandbox.
type LaunchSpec struct {
	Command        string
	Cwd            string
	AdditionalDirs []string
	Env            []string
}

// Provider manages the lifecycle of an execution sandbox.
type Provider interface {
	Backend() Backend
	Prepare(ctx context.Context, req PrepareRequest) (Prepared, error)
	SyncToRuntime(ctx context.Context, state SessionState, opts SyncOptions) (SyncResult, error)
	SyncFromRuntime(ctx context.Context, state SessionState, opts SyncOptions) (SyncResult, error)
	Destroy(ctx context.Context, state SessionState) error
}

// Finder is optionally implemented by remote providers that can discover
// provider resources by daemon-owned identity labels.
type Finder interface {
	FindSandbox(ctx context.Context, req FindSandboxRequest) (SessionState, error)
}

// Launcher starts an ACP-capable agent process inside a sandbox.
type Launcher interface {
	Launch(ctx context.Context, spec LaunchSpec) (Handle, error)
}

// Handle represents a running agent process.
type Handle interface {
	PID() int
	Cwd() string
	Stdin() io.WriteCloser
	Stdout() io.ReadCloser
	Stderr() string
	Done() <-chan struct{}
	Wait() error
	Stop(ctx context.Context) error
}

// PermissionOperation identifies a ToolHost operation subject to policy.
type PermissionOperation string

const (
	// PermissionOperationReadTextFile authorizes ACP text file reads.
	PermissionOperationReadTextFile PermissionOperation = "fs/read_text_file"
	// PermissionOperationWriteTextFile authorizes ACP text file writes.
	PermissionOperationWriteTextFile PermissionOperation = "fs/write_text_file"
	// PermissionOperationCreateTerminal authorizes terminal creation.
	PermissionOperationCreateTerminal PermissionOperation = "terminal/create"
	// PermissionOperationRequestToolGrant authorizes interactive permission requests.
	PermissionOperationRequestToolGrant PermissionOperation = "session/request_permission"
)

// PermissionDecision is a daemon policy decision for an ACP permission request.
type PermissionDecision string

const (
	// PermissionDecisionPending asks an operator or client to decide.
	PermissionDecisionPending PermissionDecision = "pending"
	// PermissionDecisionAllowOnce permits one operation.
	PermissionDecisionAllowOnce PermissionDecision = "allow-once"
	// PermissionDecisionAllowAlways permits this class of operation persistently.
	PermissionDecisionAllowAlways PermissionDecision = "allow-always"
	// PermissionDecisionRejectOnce rejects one operation.
	PermissionDecisionRejectOnce PermissionDecision = "reject-once"
	// PermissionDecisionRejectAlways rejects this class of operation persistently.
	PermissionDecisionRejectAlways PermissionDecision = "reject-always"
)

// ToolHost abstracts ACP file, permission, and terminal operations for a runtime.
type ToolHost interface {
	ReadTextFile(ctx context.Context, path string) (string, error)
	WriteTextFile(ctx context.Context, path string, content string) error
	ResolvePath(path string) (string, error)
	Authorize(op PermissionOperation) error
	PermissionDecision(req acpsdk.RequestPermissionRequest) (PermissionDecision, bool)
	CreateTerminal(ctx context.Context, req acpsdk.CreateTerminalRequest) (acpsdk.CreateTerminalResponse, error)
	KillTerminal(id string) error
	TerminalOutput(id string) (string, error)
	WaitForTerminalExit(ctx context.Context, id string) (int, error)
	ReleaseTerminal(id string) error
}
