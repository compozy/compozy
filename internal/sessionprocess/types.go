// Package sessionprocess tracks session-owned OS processes independently of agent lifetime.
package sessionprocess

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/procutil"
)

const attributionLineage = "lineage"

// Process kinds: tool calls run in the agent's turn; background and terminal work outlives it.
const (
	kindBackground = "background"
	kindTool       = "tool"
	kindTerminal   = "terminal"
)

type Identity struct {
	PID       int
	StartedAt time.Time
}

type Owner struct {
	SessionID, WorkspaceID, ProfileID, Name string
	LeaderPID                               int
	LeaderStartedAt                         time.Time
	LeaderKernelToken                       string
	Live, ToolBusy, Starting                bool
}

// Observation deliberately has no session foreign key: deleting a session keeps its processes discoverable.
type Observation struct {
	Identity
	Owner           Owner
	PGID            int
	FirstSeen       time.Time
	GroupObservedAt time.Time
	Attribution     string
	KernelToken     string
}

type Process struct {
	procutil.ScannedProcess
	Owner                         Owner
	Attribution, Kind, TerminalID string
	// Tree is the identity of the tree's root; TreeRole is lead, wrapper, or helper.
	Tree     Identity
	TreeRole string
}

type Snapshot struct {
	Supported bool
	ScannedAt time.Time
	Duration  time.Duration
	Processes []Process
}

type Summary struct {
	Supported             bool
	Background, Terminals int
	// Trees counts the Background and Terminal trees: what the session left running.
	Trees int
	Ports []int
}

type Terminal struct {
	PID       int
	StartedAt time.Time
	ID        string
	Owner     Owner
}

type Result struct {
	Identity
	Outcome, Error string
}

type Store interface {
	LoadProcessObservations(context.Context) ([]Observation, error)
	SaveProcessObservations(context.Context, []Observation, []Identity) error
}

type Source interface {
	ProcessActivity() (live bool, toolBusy bool)
	Owners(context.Context) ([]Owner, error)
	Owner(context.Context, string) (Owner, bool, error)
	OrphanOwner(context.Context, procutil.ScannedProcess) (Owner, bool, error)
	Terminals(context.Context) ([]Terminal, error)
	CloseTerminal(context.Context, Process) error
	ProcessesChanged(context.Context, string)
	RecordTermination(context.Context, Process, Result) error
}
