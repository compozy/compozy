package terminal

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/store"
	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type ID string
type Mode string
type Signal string
type ActorKind string

// OperatorActorID is the stable human-controller identity used across transports.
const OperatorActorID = "operator"

type LeaseState string
type EventKind string
type InputRequestID string

const (
	ModePTY  Mode = "pty"
	ModePipe Mode = "pipe"

	terminalAccessRead   = "read"
	terminalAccessWrite  = "write"
	terminalStateRunning = "running"
	terminalViewScreen   = "screen"
	terminalViewTail     = "tail"
	terminalWaitExit     = "exit"
	terminalWaitIdle     = "idle"
	terminalWaitMatch    = "match"

	SignalINT  Signal = "INT"
	SignalTERM Signal = "TERM"
	SignalKILL Signal = "KILL"
	SignalHUP  Signal = "HUP"

	ActorKindHuman  ActorKind = "human"
	ActorKindAgent  ActorKind = "agent"
	ActorKindSystem ActorKind = "system"

	LeaseHumanOwned LeaseState = "human_owned"
	LeaseAgentOwned LeaseState = "agent_owned"
	LeaseAvailable  LeaseState = "available"
)

type Actor struct {
	Kind       ActorKind `json:"kind"`
	ID         string    `json:"id"`
	ProfileID  string    `json:"profile_id"`
	SessionID  string    `json:"session_id,omitempty"`
	RunID      string    `json:"run_id,omitempty"`
	Generation int64     `json:"generation,omitempty"`
}

type Capabilities struct {
	Interactive bool `json:"interactive"`
}

type OpenRequest struct {
	WS           string
	Cwd          string
	Shell        string
	Title        string
	Cols         uint16
	Rows         uint16
	Actor        Actor
	Capabilities Capabilities
}

type ExecRequest struct {
	WS           string
	Command      string
	Args         []string
	Cwd          string
	Env          map[string]string
	YieldMs      int
	Visible      bool
	Output       OutputShape
	Approval     string
	Actor        Actor
	Capabilities Capabilities
}

type OutputShape struct {
	MaxBytes int    `json:"max_bytes,omitempty"`
	Strategy string `json:"strategy,omitempty"`
	Grep     string `json:"grep,omitempty"`
}

type ExecResult struct {
	ExitCode     *int      `json:"exit_code"`
	Signal       *string   `json:"signal"`
	Output       string    `json:"output"`
	Truncated    bool      `json:"truncated"`
	Untrusted    bool      `json:"untrusted"`
	Spill        *SpillRef `json:"spill,omitempty"`
	DurationMs   int64     `json:"duration_ms"`
	CommandID    string    `json:"command_id"`
	StillRunning bool      `json:"still_running,omitempty"`
	TerminalID   *ID       `json:"terminal_id"`
}

type SpillRef struct {
	ArtifactID string `json:"artifact_id"`
	Path       string `json:"-"`
	ProfileID  string `json:"-"`
	Bytes      int64  `json:"bytes"`
}

type RunRef struct {
	SessionID  string `json:"session_id"`
	RunID      string `json:"run_id"`
	Generation int64  `json:"generation"`
}

type Exit struct {
	Cause  string    `json:"cause"`
	Code   *int      `json:"code,omitempty"`
	Signal *string   `json:"signal,omitempty"`
	At     time.Time `json:"at"`
}

type Info struct {
	ID               ID           `json:"id"`
	WS               string       `json:"workspace_id"`
	ProfileID        string       `json:"profile_id"`
	Title            string       `json:"title"`
	Shell            string       `json:"shell"`
	Cwd              string       `json:"cwd"`
	Mode             Mode         `json:"mode"`
	State            string       `json:"state"`
	Controller       *Actor       `json:"controller,omitempty"`
	Lease            LeaseState   `json:"lease"`
	Viewers          int          `json:"viewers"`
	BoundRun         *RunRef      `json:"bound_run,omitempty"`
	Capabilities     Capabilities `json:"capabilities"`
	CreatedAt        time.Time    `json:"created_at"`
	Exit             *Exit        `json:"exit,omitempty"`
	TypingGeneration uint64       `json:"-"`
}

type AttachOptions struct {
	Mode     string
	Flow     string
	AfterSeq uint64
	Cols     uint16
	Rows     uint16
	Actor    Actor
}

// AttachTicketBinding is the complete immutable scope of one attach grant.
type AttachTicketBinding struct {
	WorkspaceID string
	ProfileID   string
	TerminalID  ID
	Mode        string
}

// AttachTicket is a short-lived, single-use grant minted by the terminal domain.
type AttachTicket struct {
	Token     string
	Binding   AttachTicketBinding
	Actor     Actor
	ExpiresAt time.Time
}

type ReadOptions struct {
	View     string
	MaxBytes int
	SinceSeq uint64
	FromLine int
	ToLine   int
	Grep     string
}

type ReadResult struct {
	Content   string          `json:"content"`
	Segments  []OutputSegment `json:"segments,omitempty"`
	Seq       uint64          `json:"seq"`
	Truncated bool            `json:"truncated"`
	Busy      bool            `json:"busy"`
	Untrusted bool            `json:"untrusted"`
	Spill     *SpillRef       `json:"spill,omitempty"`
}

type WaitCondition struct {
	Until     string
	Pattern   string
	TimeoutMs int
}

type WaitResult struct {
	Reason    string `json:"reason"`
	ExitCode  *int   `json:"exit_code,omitempty"`
	Screen    string `json:"screen"`
	Untrusted bool   `json:"untrusted"`
}

type InputRequest struct {
	Reason        string
	PromptExcerpt string
	Redact        bool
}

type InputOutcome struct {
	Outcome        InputResolutionOutcome `json:"outcome"`
	Redacted       bool                   `json:"redacted"`
	Length         int                    `json:"length"`
	DeliveredBytes int                    `json:"-"`
}

// InputActorProjection identifies an input requester or resolver without exposing run authority.
type InputActorProjection struct {
	Kind ActorKind `json:"kind"`
	ID   string    `json:"id"`
}

type InputAnswer struct {
	Input []byte
}

// PendingInputRequest is the profile-scoped public projection of one unresolved prompt.
type PendingInputRequest struct {
	ID            InputRequestID       `json:"id"`
	TerminalID    ID                   `json:"terminal_id"`
	WorkspaceID   string               `json:"workspace_id,omitempty"`
	ProfileID     string               `json:"profile_id"`
	ProfileName   string               `json:"profile_name"`
	Reason        string               `json:"reason"`
	PromptExcerpt string               `json:"prompt_excerpt"`
	Redacted      bool                 `json:"redacted"`
	RequestedAt   time.Time            `json:"requested_at"`
	Requester     InputActorProjection `json:"requester"`
}

// ResolvedInputRequest is the bounded, secret-free outcome projection of an input request.
type ResolvedInputRequest struct {
	ID          InputRequestID         `json:"id"`
	TerminalID  ID                     `json:"terminal_id"`
	WorkspaceID string                 `json:"workspace_id,omitempty"`
	ProfileID   string                 `json:"profile_id"`
	ProfileName string                 `json:"profile_name"`
	Requester   InputActorProjection   `json:"requester"`
	Outcome     InputResolutionOutcome `json:"outcome"`
	ResolvedBy  InputActorProjection   `json:"resolved_by"`
	Reason      string                 `json:"reason,omitempty"`
	Redacted    bool                   `json:"redacted"`
	Length      int                    `json:"length"`
	RequestedAt time.Time              `json:"requested_at"`
	ResolvedAt  time.Time              `json:"resolved_at"`
}

type RecordingRef struct {
	ID         string     `json:"id"`
	State      string     `json:"state,omitempty"`
	TerminalID ID         `json:"terminal_id"`
	ProfileID  string     `json:"profile_id"`
	Digest     string     `json:"digest"`
	Path       string     `json:"-"`
	StartedAt  time.Time  `json:"started_at"`
	StoppedAt  *time.Time `json:"stopped_at,omitempty"`
	Bytes      int64      `json:"bytes"`
	ExpiresAt  time.Time  `json:"expires_at"`
}

type Frame = terminalwire.Frame

type CommandRow struct {
	ID          string          `json:"command_id"`
	TerminalID  *ID             `json:"terminal_id"`
	ProfileID   string          `json:"profile_id"`
	ProfileName string          `json:"profile_name"`
	Actor       Actor           `json:"actor"`
	Command     string          `json:"command"`
	ArgvDigest  *string         `json:"argv_digest,omitempty"`
	Cwd         string          `json:"cwd"`
	StartedAt   time.Time       `json:"started_at"`
	DurationMs  *int64          `json:"duration_ms"`
	ExitCode    *int            `json:"exit_code"`
	ExitSignal  *string         `json:"signal"`
	ExitCause   string          `json:"exit_cause"`
	DetectedBy  string          `json:"detected_by"`
	Approval    string          `json:"approval"`
	OutputBytes int64           `json:"output_bytes"`
	Truncated   bool            `json:"truncated"`
	RecordingID *string         `json:"recording,omitempty"`
	OutputTail  []OutputSegment `json:"output_tail"`
}

type Query struct {
	Actor    string
	Since    string
	Terminal string
	Failed   bool
	Limit    int
	Cursor   string
}

type Page struct {
	Entries []CommandRow `json:"entries"`
	Next    string       `json:"next,omitempty"`
}

type FilterResult struct {
	DisplayBytes []byte
	MarkerFacts  []MarkerFacts
}

type MarkerFacts struct {
	Kind    string
	Command string
	Cwd     string
	Exit    *int
}

type OutputSegmentKind string

const (
	OutputSegmentBytes         OutputSegmentKind = "output"
	OutputSegmentRedactedInput OutputSegmentKind = "redacted_input"
)

type OutputSegment struct {
	Kind       OutputSegmentKind `json:"kind"`
	Text       string            `json:"text,omitempty"`
	Characters int               `json:"characters,omitempty"`
}

type Manager interface {
	Open(ctx context.Context, request OpenRequest) (Handle, error)
	Exec(ctx context.Context, request ExecRequest) (*ExecResult, error)
	Handle(ctx context.Context, workspaceID, profileID string, id ID) (Handle, error)
	Get(ctx context.Context, workspaceID, profileID string, id ID) (*Info, error)
	List(ctx context.Context, workspaceID string, scope store.ReadScope) ([]Info, error)
	ActiveRecordings(ctx context.Context, workspaceID string, scope store.ReadScope) ([]RecordingRef, error)
	Close(ctx context.Context, workspaceID string, id ID, actor Actor, signal Signal) (*Exit, error)
	Capabilities(ctx context.Context, workspaceID string) (Capabilities, error)
	MintAttachTicket(ctx context.Context, binding AttachTicketBinding, actor Actor) (AttachTicket, error)
	AttachWithTicket(
		ctx context.Context,
		token string,
		workspaceID string,
		terminalID ID,
		mode string,
		options AttachOptions,
	) (Handle, Subscription, AttachTicket, error)
	Claim(ctx context.Context, workspaceID string, id ID, actor Actor) error
	RunEnded(ctx context.Context, workspaceID string, actor Actor) int
	SessionRunEnded(ctx context.Context, workspaceID, profileID, sessionID, runID string, generation int64) int
	RuntimeRecovered(ctx context.Context, workspaceID string, previous, current Actor) int
	InputRequests(
		ctx context.Context,
		workspaceID string,
		scope store.ReadScope,
		terminalID ID,
	) ([]PendingInputRequest, error)
	ResolvedInputRequests(
		ctx context.Context,
		workspaceID string,
		scope store.ReadScope,
		terminalID ID,
	) ([]ResolvedInputRequest, error)
	Journal() Journal
	Shutdown(ctx context.Context) error
	Observe(fn func(context.Context, Event))
	ArchiveProfile(ctx context.Context, profileID string) error
	ArchiveWorkspace(ctx context.Context, workspaceID string) error
	PrepareWorkspaceRemoval(
		ctx context.Context,
		workspaceID string,
	) (workspacepkg.UnregisterPreparation, error)
}

type ProfileGuard interface {
	EnsureAvailableID(ctx context.Context, profileID string) error
}
