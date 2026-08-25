package terminal

import (
	"context"
	"io"
	"time"

	"github.com/compozy/compozy/internal/store"
	terminalpty "github.com/compozy/compozy/internal/terminal/pty"
)

type ID string
type Mode string
type Signal string
type ActorKind string
type LeaseState string
type EventKind string
type InputRequestID string

const (
	ModePTY  Mode = "pty"
	ModePipe Mode = "pipe"

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
	Kind       ActorKind
	ID         string
	ProfileID  string
	SessionID  string
	RunID      string
	Generation int64
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
	Actor        Actor
	Capabilities Capabilities
}

type OutputShape struct {
	MaxBytes int
	Strategy string
	Grep     string
}

type ExecResult struct {
	ExitCode     *int
	Signal       *string
	Output       string
	Truncated    bool
	Untrusted    bool
	Spill        *SpillRef
	DurationMs   int64
	CommandID    string
	StillRunning bool
	TerminalID   *ID
}

type SpillRef struct {
	ArtifactID string
	Path       string
	ProfileID  string
	Bytes      int64
}

type RunRef struct {
	SessionID  string
	RunID      string
	Generation int64
}

type Exit struct {
	Cause  string
	Code   *int
	Signal *string
	At     time.Time
}

type Info struct {
	ID           ID
	WS           string
	ProfileID    string
	Title        string
	Shell        string
	Cwd          string
	Mode         Mode
	State        string
	Controller   *Actor
	Lease        LeaseState
	Viewers      int
	BoundRun     *RunRef
	Capabilities Capabilities
	CreatedAt    time.Time
	Exit         *Exit
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
	Content   string
	Seq       uint64
	Truncated bool
	Busy      bool
	Untrusted bool
	Spill     *SpillRef
}

type WaitCondition struct {
	Until     string
	Pattern   string
	TimeoutMs int
}

type WaitResult struct {
	Reason    string
	ExitCode  *int
	Screen    string
	Untrusted bool
}

type InputRequest struct {
	Reason        string
	PromptExcerpt string
	Redact        bool
}

type InputOutcome struct {
	Outcome  string
	Redacted bool
	Length   int
}

type InputAnswer struct {
	Input []byte
}

type RecordingRef struct {
	ID         string
	TerminalID ID
	ProfileID  string
	Digest     string
	Path       string
	StartedAt  time.Time
	StoppedAt  *time.Time
	Bytes      int64
	ExpiresAt  time.Time
}

type Frame struct {
	Op      byte
	Seq     uint64
	Payload []byte
}

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
	Entries []CommandRow
	Next    string
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
	AnswerInput(ctx context.Context, actor Actor, id InputRequestID, answer InputAnswer) error
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
	Observe(fn func(context.Context, TerminalEvent))
	ArchiveProfile(ctx context.Context, profileID string) error
}

type Journal interface {
	Record(ctx context.Context, workspaceID string, row CommandRow) error
	Query(ctx context.Context, workspaceID string, scope store.ReadScope, query Query) (*Page, error)
	LinkRecording(ctx context.Context, workspaceID string, terminalID ID, recording RecordingRef) error
	Recording(ctx context.Context, workspaceID string, scope store.ReadScope, id string) (*RecordingRef, io.ReadCloser, error)
	Artifact(ctx context.Context, workspaceID string, scope store.ReadScope, id string) (io.ReadCloser, error)
}

type ProfileGuard interface {
	EnsureAvailableID(ctx context.Context, profileID string) error
}
