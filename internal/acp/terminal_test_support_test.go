package acp

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/store"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/compozy/compozy/internal/toolruntime"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type acpTestTerminalJournal struct{}

func (acpTestTerminalJournal) ReserveCommandID(context.Context, string) (string, error) {
	return "cmd-0000000000000001", nil
}

func (acpTestTerminalJournal) ReleaseCommandID(string, string) {}

func (acpTestTerminalJournal) ReserveRecordingID(context.Context, string) (string, error) {
	return "rec-0000000000000001", nil
}

func (acpTestTerminalJournal) ReleaseRecordingID(string, string) {}

func (acpTestTerminalJournal) Record(context.Context, string, terminalpkg.CommandRow) error {
	return nil
}

func (j acpTestTerminalJournal) RecordQueued(
	ctx context.Context,
	info terminalpkg.Info,
	row terminalpkg.CommandRow,
) error {
	return j.Record(ctx, info.WS, row)
}

func (acpTestTerminalJournal) Query(
	context.Context,
	string,
	store.ReadScope,
	terminalpkg.Query,
) (*terminalpkg.Page, error) {
	return &terminalpkg.Page{}, nil
}

func (acpTestTerminalJournal) LinkRecording(
	context.Context,
	string,
	terminalpkg.ID,
	terminalpkg.RecordingRef,
) error {
	return nil
}

func (acpTestTerminalJournal) Recording(
	context.Context,
	string,
	store.ReadScope,
	string,
) (*terminalpkg.RecordingRef, io.ReadCloser, error) {
	return nil, nil, errors.New("acp test: recording not found")
}

func (acpTestTerminalJournal) Artifact(
	context.Context,
	string,
	store.ReadScope,
	string,
) (io.ReadCloser, error) {
	return nil, errors.New("acp test: artifact not found")
}

func (acpTestTerminalJournal) RemoveWorkspace(context.Context, string) error { return nil }

func (acpTestTerminalJournal) PrepareWorkspaceRemoval(
	context.Context,
	string,
) (workspacepkg.UnregisterPreparation, error) {
	return acpWorkspaceRemovalPreparation{}, nil
}

func (acpTestTerminalJournal) PrepareWorkspaceRemovalAt(
	context.Context,
	string,
	string,
) (workspacepkg.UnregisterPreparation, error) {
	return acpWorkspaceRemovalPreparation{}, nil
}

func (acpTestTerminalJournal) ConsumeMarkerFacts(
	context.Context,
	terminalpkg.Info,
	[]terminalpkg.MarkerFacts,
) error {
	return nil
}

func (acpTestTerminalJournal) RegisterTerminal(
	terminalpkg.Info,
	func(bool),
	func(terminalpkg.Event),
) {
}

func (acpTestTerminalJournal) CloseTerminal(context.Context, terminalpkg.Info) error { return nil }

func (acpTestTerminalJournal) ReserveInput(
	terminalpkg.Info,
	terminalpkg.JournalInput,
) (terminalpkg.JournalInputReservation, bool) {
	return acpTestJournalReservation{}, true
}

func (acpTestTerminalJournal) ObserveOutput(terminalpkg.Info, []byte) {}
func (acpTestTerminalJournal) Shutdown(context.Context) error         { return nil }

func (acpTestTerminalJournal) PersistRecording(
	context.Context,
	string,
	terminalpkg.ID,
	terminalpkg.RecordingRef,
	[]byte,
) (terminalpkg.RecordingRef, error) {
	return terminalpkg.RecordingRef{}, nil
}

func (acpTestTerminalJournal) WriteArtifact(
	context.Context,
	string,
	string,
	string,
	*terminalpkg.ID,
	[]byte,
	time.Time,
) (terminalpkg.SpillRef, error) {
	return terminalpkg.SpillRef{}, nil
}

type acpTestJournalReservation struct{}

type acpWorkspaceRemovalPreparation struct{}

func (acpWorkspaceRemovalPreparation) BeforeDelete(context.Context) error { return nil }
func (acpWorkspaceRemovalPreparation) Commit(context.Context) error       { return nil }
func (acpWorkspaceRemovalPreparation) Rollback(context.Context) error     { return nil }

func (acpTestJournalReservation) Commit(terminalpkg.Actor, terminalpkg.JournalInput) {}
func (acpTestJournalReservation) Release()                                           {}

func newACPTestTerminalCore(
	t testing.TB,
	registry *toolruntime.Registry,
) *terminalpkg.Service {
	t.Helper()
	options := []terminalpkg.Option{terminalpkg.WithJournal(acpTestTerminalJournal{})}
	if registry != nil {
		options = append(options, terminalpkg.WithProcessRegistry(registry))
	}
	manager, err := terminalpkg.NewManager(options...)
	if err != nil {
		t.Fatalf("terminal.NewManager() error = %v", err)
	}
	if err := manager.Start(t.Context()); err != nil {
		t.Fatalf("terminal.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(context.WithoutCancel(t.Context())); err != nil {
			t.Errorf("terminal.Shutdown() error = %v", err)
		}
	})
	return manager
}

func withACPTestTerminalCore(t testing.TB, registry *toolruntime.Registry) LocalRuntimeOption {
	t.Helper()
	return WithLocalTerminalManager(newACPTestTerminalCore(t, registry), LocalTerminalScope{
		WorkspaceID: "acp-test", ProfileID: "acp-test", ActorID: "acp-test", Generation: 1,
	})
}

func beginACPTestRun(t testing.TB, process *AgentProcess, turnID string) *activePromptState {
	t.Helper()
	active, err := process.beginPromptForRun(turnID, "run-"+turnID, 1, 8)
	if err != nil {
		t.Fatalf("beginPromptForRun() error = %v", err)
	}
	t.Cleanup(func() { process.endPrompt(active) })
	return active
}

func createACPTestTerminal(
	ctx context.Context,
	host *localToolHost,
	request acpsdk.CreateTerminalRequest,
) (acpsdk.CreateTerminalResponse, error) {
	return host.createTerminal(ctx, request, terminalOwnership{
		ownerSessionID:  string(request.SessionId),
		ownerTurnID:     "turn-test-terminal",
		ownerRunID:      "run-test-terminal",
		ownerGeneration: 1,
	})
}
