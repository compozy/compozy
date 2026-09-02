package terminal

import (
	"context"
	"io"
	"time"

	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type MarkerConsumer interface {
	ConsumeMarkerFacts(ctx context.Context, terminal Info, facts []MarkerFacts) error
}

type JournalInput struct {
	Content    []byte
	Redacted   bool
	Characters int
}

// JournalInputReservation pins one terminal journal lane across PTY delivery.
type JournalInputReservation interface {
	Commit(actor Actor, input JournalInput)
	Release()
}

type Journal interface {
	MarkerConsumer
	ReserveCommandID(ctx context.Context, workspaceID string) (string, error)
	ReleaseCommandID(workspaceID, commandID string)
	ReserveRecordingID(ctx context.Context, workspaceID string) (string, error)
	ReleaseRecordingID(workspaceID, recordingID string)
	Record(ctx context.Context, workspaceID string, row CommandRow) error
	RecordQueued(ctx context.Context, terminal Info, row CommandRow) error
	Query(ctx context.Context, workspaceID string, scope store.ReadScope, query Query) (*Page, error)
	LinkRecording(ctx context.Context, workspaceID string, terminalID ID, recording RecordingRef) error
	PersistRecording(
		ctx context.Context,
		workspaceID string,
		terminalID ID,
		recording RecordingRef,
		contents []byte,
	) (RecordingRef, error)
	WriteArtifact(
		ctx context.Context,
		workspaceID string,
		profileID string,
		commandID string,
		terminalID *ID,
		contents []byte,
		expiresAt time.Time,
	) (SpillRef, error)
	Recording(
		ctx context.Context,
		workspaceID string,
		scope store.ReadScope,
		id string,
	) (*RecordingRef, io.ReadCloser, error)
	Artifact(ctx context.Context, workspaceID string, scope store.ReadScope, id string) (io.ReadCloser, error)
	RegisterTerminal(terminal Info, setBlocked func(bool), emit func(Event))
	CloseTerminal(ctx context.Context, terminal Info) error
	ReserveInput(terminal Info, input JournalInput) (JournalInputReservation, bool)
	ObserveOutput(terminal Info, output []byte)
	PrepareWorkspaceRemoval(
		ctx context.Context,
		workspaceID string,
	) (workspacepkg.UnregisterPreparation, error)
	PrepareWorkspaceRemovalAt(
		ctx context.Context,
		workspaceID string,
		rootDir string,
	) (workspacepkg.UnregisterPreparation, error)
	RemoveWorkspace(ctx context.Context, workspaceID string) error
	Shutdown(ctx context.Context) error
}
