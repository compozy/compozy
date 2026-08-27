package terminal

import (
	"context"
	"io"
	"time"

	"github.com/compozy/compozy/internal/store"
	terminalpty "github.com/compozy/compozy/internal/terminal/pty"
	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
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

type ReadOptions struct {
	View     string
	MaxBytes int
	SinceSeq uint64
	FromLine int
	ToLine   int
	Grep     string
}

type ReadResult struct {
	Content   string    `json:"content"`
	Seq       uint64    `json:"seq"`
	Truncated bool      `json:"truncated"`
	Busy      bool      `json:"busy"`
	Untrusted bool      `json:"untrusted"`
	Spill     *SpillRef `json:"spill,omitempty"`
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
	Outcome  string `json:"outcome"`
	Redacted bool   `json:"redacted"`
	Length   int    `json:"length"`
}

type InputAnswer struct {
	Input []byte
}

// PendingInputRequest is the profile-scoped public projection of one unresolved prompt.
type PendingInputRequest struct {
	ID            InputRequestID `json:"id"`
	TerminalID    ID             `json:"terminal_id"`
	WorkspaceID   string         `json:"workspace_id,omitempty"`
	ProfileID     string         `json:"profile_id"`
	ProfileName   string         `json:"profile_name"`
	Reason        string         `json:"reason"`
	PromptExcerpt string         `json:"prompt_excerpt"`
	Redacted      bool           `json:"redacted"`
	RequestedAt   time.Time      `json:"requested_at"`
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
	ID          string    `json:"command_id"`
	TerminalID  *ID       `json:"terminal_id"`
	ProfileID   string    `json:"profile_id"`
	ProfileName string    `json:"profile_name"`
	Actor       Actor     `json:"actor"`
	Command     string    `json:"command"`
	ArgvDigest  *string   `json:"argv_digest,omitempty"`
	Cwd         string    `json:"cwd"`
	StartedAt   time.Time `json:"started_at"`
	DurationMs  *int64    `json:"duration_ms"`
	ExitCode    *int      `json:"exit_code"`
	ExitSignal  *string   `json:"signal"`
	ExitCause   string    `json:"exit_cause"`
	DetectedBy  string    `json:"detected_by"`
	Approval    string    `json:"approval"`
	OutputBytes int64     `json:"output_bytes"`
	Truncated   bool      `json:"truncated"`
	RecordingID *string   `json:"recording,omitempty"`
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

type MarkerConsumer interface {
	ConsumeMarkerFacts(ctx context.Context, terminal Info, facts []MarkerFacts) error
}

type ProcSpec = terminalpty.ProcSpec
type PTY = terminalpty.PTY
type Proc = terminalpty.Proc

type Subscription interface {
	Frames() <-chan Frame
	Ack(bytes int)
	Resize(cols, rows uint16) error
	Close() error
}

type Handle interface {
	Info() Info
	MarkerNonce() string
	Attach(ctx context.Context, options AttachOptions) (Subscription, error)
	Write(ctx context.Context, actor Actor, input []byte) error
	Screen(ctx context.Context, options ReadOptions) (*ReadResult, error)
	Wait(ctx context.Context, condition WaitCondition) (*WaitResult, error)
	Takeover(ctx context.Context, actor Actor, force bool) error
	Yield(ctx context.Context, actor Actor) error
	RequestInput(ctx context.Context, request InputRequest) (*InputOutcome, error)
	AnswerInput(ctx context.Context, actor Actor, id InputRequestID, answer InputAnswer) (*InputOutcome, error)
	RejectInput(ctx context.Context, actor Actor, id InputRequestID, reason string) error
	Signal(ctx context.Context, actor Actor, signal Signal) error
	StartRecording(ctx context.Context, actor Actor) (RecordingRef, error)
	StopRecording(ctx context.Context, actor Actor) (RecordingRef, error)
}

type Manager interface {
	Open(ctx context.Context, request OpenRequest) (Handle, error)
	Exec(ctx context.Context, request ExecRequest) (*ExecResult, error)
	Handle(ctx context.Context, workspaceID, profileID string, id ID) (Handle, error)
	Get(ctx context.Context, workspaceID, profileID string, id ID) (*Info, error)
	List(ctx context.Context, workspaceID string, scope store.ReadScope) ([]Info, error)
	Close(ctx context.Context, workspaceID string, id ID, actor Actor, signal Signal) (*Exit, error)
	Journal() Journal
	Shutdown(ctx context.Context) error
	Observe(fn func(context.Context, Event))
	ArchiveProfile(ctx context.Context, profileID string) error
}

type Journal interface {
	Record(ctx context.Context, workspaceID string, row CommandRow) error
	Query(ctx context.Context, workspaceID string, scope store.ReadScope, query Query) (*Page, error)
	LinkRecording(ctx context.Context, workspaceID string, terminalID ID, recording RecordingRef) error
	Recording(
		ctx context.Context,
		workspaceID string,
		scope store.ReadScope,
		id string,
	) (*RecordingRef, io.ReadCloser, error)
	Artifact(ctx context.Context, workspaceID string, scope store.ReadScope, id string) (io.ReadCloser, error)
}

type ProfileGuard interface {
	EnsureAvailableID(ctx context.Context, profileID string) error
}
