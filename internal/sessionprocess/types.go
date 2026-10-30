// Package sessionprocess tracks session-owned OS processes independently of agent lifetime.
package sessionprocess

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/procutil"
)

const attributionLineage = "lineage"

type Identity struct {
	PID       int
	StartedAt time.Time
}

type Owner struct {
	SessionID, WorkspaceID, ProfileID, Name string
	LeaderPID                               int
	LeaderStartedAt                         time.Time
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
}

type Process struct {
	procutil.ScannedProcess
	Owner                         Owner
	Attribution, Kind, TerminalID string
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
	Ports                 []int
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
	Owners(context.Context) ([]Owner, error)
	Owner(context.Context, string) (Owner, bool, error)
	OrphanOwner(context.Context, procutil.ScannedProcess) (Owner, bool, error)
	Terminals(context.Context) ([]Terminal, error)
	CloseTerminal(context.Context, Process) error
	ProcessesChanged(context.Context, string)
	RecordTermination(context.Context, Process, Result) error
}
