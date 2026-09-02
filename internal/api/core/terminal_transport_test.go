package core

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
	"github.com/compozy/compozy/internal/windowmanager"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gorilla/websocket"
)

type terminalProviderStub struct {
	terminalpkg.Manager
	observers []func(context.Context, terminalpkg.Event)
}

type terminalWindowManagerProviderStub struct {
	WindowManagerProvider
	service WindowManagerService
}

func (p *terminalWindowManagerProviderStub) WindowManagerFor(string) (WindowManagerService, error) {
	return p.service, nil
}

type terminalWindowManagerServiceStub struct {
	WindowManagerService
	clientID string
	token    string
	err      error
}

func (s *terminalWindowManagerServiceStub) AuthorizeClient(
	_ context.Context,
	_ windowmanager.WorkspaceID,
	clientID windowmanager.ClientID,
	token string,
) error {
	s.clientID = string(clientID)
	s.token = token
	return s.err
}

func (p *terminalProviderStub) Observe(observer func(context.Context, terminalpkg.Event)) {
	p.observers = append(p.observers, observer)
}

func (p *terminalProviderStub) emit(event terminalpkg.Event) {
	for _, observer := range p.observers {
		observer(context.Background(), event)
	}
}

type terminalManagerStub struct {
	handle               terminalpkg.Handle
	info                 *terminalpkg.Info
	infos                []terminalpkg.Info
	mintErr              error
	mintedActor          *terminalpkg.Actor
	activeRecordings     []terminalpkg.RecordingRef
	activeRecordingQuery *terminalRecordingQuery
}

type terminalRecordingQuery struct {
	workspaceID string
	scope       store.ReadScope
	cancel      context.CancelFunc
}

type terminalOpenManagerStub struct {
	terminalManagerStub
	request terminalpkg.OpenRequest
}

func (m *terminalOpenManagerStub) Open(
	_ context.Context,
	request terminalpkg.OpenRequest,
) (terminalpkg.Handle, error) {
	m.request = request
	return terminalHandleStub{info: terminalpkg.Info{
		ID: "term-a", WS: request.WS, ProfileID: request.Actor.ProfileID,
	}}, nil
}

type terminalAgentManagerStub struct {
	terminalManagerStub
	handle     *terminalAgentHandleStub
	journal    *terminalAgentJournalStub
	exec       terminalpkg.ExecRequest
	inputScope store.ReadScope
}

func (m *terminalAgentManagerStub) Exec(
	_ context.Context,
	request terminalpkg.ExecRequest,
) (*terminalpkg.ExecResult, error) {
	m.exec = request
	id := terminalpkg.ID("term-a")
	return &terminalpkg.ExecResult{StillRunning: true, TerminalID: &id, CommandID: "cmd-a", Untrusted: true}, nil
}

func (m *terminalAgentManagerStub) Handle(context.Context, string, string, terminalpkg.ID) (terminalpkg.Handle, error) {
	return m.handle, nil
}

func (m *terminalAgentManagerStub) InputRequests(
	_ context.Context,
	_ string,
	scope store.ReadScope,
	_ terminalpkg.ID,
) ([]terminalpkg.PendingInputRequest, error) {
	m.inputScope = scope
	return []terminalpkg.PendingInputRequest{{ID: "input-a"}}, nil
}

func (m *terminalAgentManagerStub) Journal() terminalpkg.Journal { return m.journal }

type terminalAgentJournalStub struct {
	scope store.ReadScope
	query terminalpkg.Query
}

func (*terminalAgentJournalStub) ReserveCommandID(context.Context, string) (string, error) {
	return "cmd-0000000000000001", nil
}

func (*terminalAgentJournalStub) ReleaseCommandID(string, string) {}

func (*terminalAgentJournalStub) ReserveRecordingID(context.Context, string) (string, error) {
	return "rec-0000000000000001", nil
}

func (*terminalAgentJournalStub) ReleaseRecordingID(string, string) {}

type terminalDownloadManagerStub struct {
	terminalManagerStub
	journal terminalpkg.Journal
}

func (m terminalDownloadManagerStub) Journal() terminalpkg.Journal { return m.journal }

type terminalDownloadJournalStub struct {
	terminalAgentJournalStub
	recordingScope store.ReadScope
	artifactScope  store.ReadScope
}

func (j *terminalDownloadJournalStub) Recording(
	_ context.Context,
	_ string,
	scope store.ReadScope,
	id string,
) (*terminalpkg.RecordingRef, io.ReadCloser, error) {
	j.recordingScope = scope
	if id == "foreign" {
		return nil, nil, os.ErrNotExist
	}
	recording := &terminalpkg.RecordingRef{
		ID:    id,
		Bytes: int64(len("asciicast")),
	}
	reader := io.NopCloser(strings.NewReader("asciicast"))
	return recording, reader, nil
}

func (j *terminalDownloadJournalStub) Artifact(
	_ context.Context,
	_ string,
	scope store.ReadScope,
	_ string,
) (io.ReadCloser, error) {
	j.artifactScope = scope
	return io.NopCloser(strings.NewReader("artifact bytes")), nil
}

func (*terminalAgentJournalStub) Record(context.Context, string, terminalpkg.CommandRow) error {
	return nil
}
func (j *terminalAgentJournalStub) RecordQueued(
	ctx context.Context,
	info terminalpkg.Info,
	row terminalpkg.CommandRow,
) error {
	return j.Record(ctx, info.WS, row)
}
func (j *terminalAgentJournalStub) Query(
	_ context.Context,
	_ string,
	scope store.ReadScope,
	query terminalpkg.Query,
) (*terminalpkg.Page, error) {
	j.scope = scope
	j.query = query
	return &terminalpkg.Page{Entries: []terminalpkg.CommandRow{}}, nil
}

func (*terminalAgentJournalStub) LinkRecording(
	context.Context,
	string,
	terminalpkg.ID,
	terminalpkg.RecordingRef,
) error {
	return nil
}
func (*terminalAgentJournalStub) Recording(
	context.Context,
	string,
	store.ReadScope,
	string,
) (*terminalpkg.RecordingRef, io.ReadCloser, error) {
	return nil, nil, terminalpkg.ErrUnsupported
}
func (*terminalAgentJournalStub) Artifact(context.Context, string, store.ReadScope, string) (io.ReadCloser, error) {
	return nil, terminalpkg.ErrUnsupported
}
func (*terminalAgentJournalStub) RemoveWorkspace(context.Context, string) error { return nil }
func (*terminalAgentJournalStub) PrepareWorkspaceRemoval(
	context.Context,
	string,
) (workspacepkg.UnregisterPreparation, error) {
	return terminalAgentWorkspaceRemovalPreparation{}, nil
}
func (*terminalAgentJournalStub) PrepareWorkspaceRemovalAt(
	context.Context,
	string,
	string,
) (workspacepkg.UnregisterPreparation, error) {
	return terminalAgentWorkspaceRemovalPreparation{}, nil
}
func (*terminalAgentJournalStub) ConsumeMarkerFacts(
	context.Context,
	terminalpkg.Info,
	[]terminalpkg.MarkerFacts,
) error {
	return nil
}
func (*terminalAgentJournalStub) RegisterTerminal(
	terminalpkg.Info,
	func(bool),
	func(terminalpkg.Event),
) {
}
func (*terminalAgentJournalStub) CloseTerminal(context.Context, terminalpkg.Info) error { return nil }
func (*terminalAgentJournalStub) ReserveInput(
	terminalpkg.Info,
	terminalpkg.JournalInput,
) (terminalpkg.JournalInputReservation, bool) {
	return terminalAgentJournalReservation{}, true
}
func (*terminalAgentJournalStub) ObserveOutput(terminalpkg.Info, []byte) {}
func (*terminalAgentJournalStub) Shutdown(context.Context) error         { return nil }
func (*terminalAgentJournalStub) PersistRecording(
	context.Context,
	string,
	terminalpkg.ID,
	terminalpkg.RecordingRef,
	[]byte,
) (terminalpkg.RecordingRef, error) {
	return terminalpkg.RecordingRef{}, nil
}
func (*terminalAgentJournalStub) WriteArtifact(
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

type terminalAgentJournalReservation struct{}

func (terminalAgentJournalReservation) Commit(terminalpkg.Actor, terminalpkg.JournalInput) {}
func (terminalAgentJournalReservation) Release()                                           {}

type scopeRecordingTerminalManager struct {
	terminalManagerStub
	scope store.ReadScope
}

func (m *scopeRecordingTerminalManager) List(
	_ context.Context,
	_ string,
	scope store.ReadScope,
) ([]terminalpkg.Info, error) {
	m.scope = scope
	return []terminalpkg.Info{}, nil
}

func (terminalManagerStub) Open(context.Context, terminalpkg.OpenRequest) (terminalpkg.Handle, error) {
	return nil, terminalpkg.ErrUnsupported
}
func (terminalManagerStub) Exec(context.Context, terminalpkg.ExecRequest) (*terminalpkg.ExecResult, error) {
	return nil, terminalpkg.ErrUnsupported
}
func (m terminalManagerStub) Handle(context.Context, string, string, terminalpkg.ID) (terminalpkg.Handle, error) {
	return m.handle, nil
}
func (m terminalManagerStub) Get(context.Context, string, string, terminalpkg.ID) (*terminalpkg.Info, error) {
	if m.info != nil {
		return m.info, nil
	}
	return &terminalpkg.Info{}, nil
}
func (m terminalManagerStub) List(context.Context, string, store.ReadScope) ([]terminalpkg.Info, error) {
	return append([]terminalpkg.Info(nil), m.infos...), nil
}

func (m terminalManagerStub) ActiveRecordings(
	_ context.Context,
	workspaceID string,
	scope store.ReadScope,
) ([]terminalpkg.RecordingRef, error) {
	if m.activeRecordingQuery != nil {
		m.activeRecordingQuery.workspaceID = workspaceID
		m.activeRecordingQuery.scope = scope
		if m.activeRecordingQuery.cancel != nil {
			m.activeRecordingQuery.cancel()
		}
	}
	return append([]terminalpkg.RecordingRef(nil), m.activeRecordings...), nil
}

func (terminalManagerStub) Capabilities(context.Context, string) (terminalpkg.Capabilities, error) {
	return terminalpkg.Capabilities{Interactive: true}, nil
}

func (m terminalManagerStub) MintAttachTicket(
	_ context.Context,
	binding terminalpkg.AttachTicketBinding,
	actor terminalpkg.Actor,
) (terminalpkg.AttachTicket, error) {
	if m.mintErr != nil {
		return terminalpkg.AttachTicket{}, m.mintErr
	}
	if m.mintedActor != nil {
		*m.mintedActor = actor
	}
	return terminalpkg.AttachTicket{
		Token: "tkt-test", Binding: binding, Actor: actor, ExpiresAt: time.Now().Add(time.Minute),
	}, nil
}

func (m terminalManagerStub) AttachWithTicket(
	ctx context.Context,
	token string,
	workspaceID string,
	terminalID terminalpkg.ID,
	mode string,
	options terminalpkg.AttachOptions,
) (terminalpkg.Handle, terminalpkg.Subscription, terminalpkg.AttachTicket, error) {
	if strings.TrimSpace(token) == "" {
		return nil, nil, terminalpkg.AttachTicket{}, &terminalpkg.Error{
			Code: terminalpkg.ErrorCodeTicketInvalid, Message: "terminal attach ticket is invalid",
			Err: terminalpkg.ErrTicketInvalid,
		}
	}
	if m.handle == nil {
		return nil, nil, terminalpkg.AttachTicket{}, terminalpkg.ErrUnsupported
	}
	subscription, err := m.handle.Attach(ctx, options)
	if err != nil {
		return nil, nil, terminalpkg.AttachTicket{}, err
	}
	ticket := terminalpkg.AttachTicket{Token: token, Binding: terminalpkg.AttachTicketBinding{
		WorkspaceID: workspaceID, TerminalID: terminalID, Mode: mode,
	}}
	return m.handle, subscription, ticket, nil
}

func (terminalManagerStub) Claim(context.Context, string, terminalpkg.ID, terminalpkg.Actor) error {
	return nil
}

func (terminalManagerStub) RunEnded(context.Context, string, terminalpkg.Actor) int { return 0 }

func (terminalManagerStub) SessionRunEnded(context.Context, string, string, string, string, int64) int {
	return 0
}

func (terminalManagerStub) RuntimeRecovered(context.Context, string, terminalpkg.Actor, terminalpkg.Actor) int {
	return 0
}

func (terminalManagerStub) InputRequests(
	context.Context,
	string,
	store.ReadScope,
	terminalpkg.ID,
) ([]terminalpkg.PendingInputRequest, error) {
	return nil, nil
}

func (terminalManagerStub) ResolvedInputRequests(
	context.Context,
	string,
	store.ReadScope,
	terminalpkg.ID,
) ([]terminalpkg.ResolvedInputRequest, error) {
	return nil, nil
}

func (terminalManagerStub) Close(
	context.Context,
	string,
	terminalpkg.ID,
	terminalpkg.Actor,
	terminalpkg.Signal,
) (*terminalpkg.Exit, error) {
	return nil, terminalpkg.ErrUnsupported
}
func (terminalManagerStub) Journal() terminalpkg.Journal                     { return nil }
func (terminalManagerStub) Shutdown(context.Context) error                   { return nil }
func (terminalManagerStub) Observe(func(context.Context, terminalpkg.Event)) {}
func (terminalManagerStub) ArchiveProfile(context.Context, string) error     { return nil }
func (terminalManagerStub) ArchiveWorkspace(context.Context, string) error   { return nil }
func (terminalManagerStub) PrepareWorkspaceRemoval(
	context.Context,
	string,
) (workspacepkg.UnregisterPreparation, error) {
	return terminalAgentWorkspaceRemovalPreparation{}, nil
}

type terminalAgentWorkspaceRemovalPreparation struct{}

func (terminalAgentWorkspaceRemovalPreparation) BeforeDelete(context.Context) error { return nil }
func (terminalAgentWorkspaceRemovalPreparation) Commit(context.Context) error       { return nil }
func (terminalAgentWorkspaceRemovalPreparation) Rollback(context.Context) error     { return nil }

type terminalHandleStub struct {
	info         terminalpkg.Info
	attachErr    error
	writeErr     error
	subscription terminalpkg.Subscription
	screenResult *terminalpkg.ReadResult
	screenErr    error
	pending      *terminalpkg.PendingInputRequest
	answer       *terminalpkg.InputOutcome
}

type terminalAgentHandleStub struct {
	terminalHandleStub
	signal       terminalpkg.Signal
	rejected     terminalpkg.InputRequestID
	recordStarts int
	recordStops  int
}

func (*terminalAgentHandleStub) Wait(context.Context, terminalpkg.WaitCondition) (*terminalpkg.WaitResult, error) {
	return &terminalpkg.WaitResult{Reason: "match", Screen: "ok", Untrusted: true}, nil
}
func (h *terminalAgentHandleStub) Signal(_ context.Context, _ terminalpkg.Actor, signal terminalpkg.Signal) error {
	h.signal = signal
	return nil
}
func (h *terminalAgentHandleStub) RejectInput(
	_ context.Context,
	_ terminalpkg.Actor,
	id terminalpkg.InputRequestID,
	_ string,
) error {
	h.rejected = id
	return nil
}
func (h *terminalAgentHandleStub) StartRecording(context.Context, terminalpkg.Actor) (terminalpkg.RecordingRef, error) {
	h.recordStarts++
	return terminalpkg.RecordingRef{ID: "recording-a"}, nil
}
func (h *terminalAgentHandleStub) StopRecording(context.Context, terminalpkg.Actor) (terminalpkg.RecordingRef, error) {
	h.recordStops++
	return terminalpkg.RecordingRef{ID: "recording-a"}, nil
}

func (h terminalHandleStub) Info() terminalpkg.Info { return h.info }
func (terminalHandleStub) MarkerNonce() string      { return "" }
func (h terminalHandleStub) Attach(context.Context, terminalpkg.AttachOptions) (terminalpkg.Subscription, error) {
	return h.subscription, h.attachErr
}
func (h terminalHandleStub) Write(context.Context, terminalpkg.Actor, []byte) error {
	return h.writeErr
}
func (h terminalHandleStub) Screen(context.Context, terminalpkg.ReadOptions) (*terminalpkg.ReadResult, error) {
	return h.screenResult, h.screenErr
}
func (terminalHandleStub) Wait(context.Context, terminalpkg.WaitCondition) (*terminalpkg.WaitResult, error) {
	return nil, nil
}
func (terminalHandleStub) Takeover(context.Context, terminalpkg.Actor, bool) error { return nil }
func (terminalHandleStub) Yield(context.Context, terminalpkg.Actor) error          { return nil }
func (terminalHandleStub) RequestInput(context.Context, terminalpkg.InputRequest) (*terminalpkg.InputOutcome, error) {
	return nil, nil
}

func (h terminalHandleStub) AnswerInput(
	context.Context,
	terminalpkg.Actor,
	terminalpkg.InputRequestID,
	terminalpkg.InputAnswer,
) (*terminalpkg.InputOutcome, error) {
	if h.answer != nil {
		return h.answer, nil
	}
	return &terminalpkg.InputOutcome{Outcome: "answered"}, nil
}
func (terminalHandleStub) RejectInput(context.Context, terminalpkg.Actor, terminalpkg.InputRequestID, string) error {
	return nil
}
func (h terminalHandleStub) PendingInput(terminalpkg.InputRequestID) (*terminalpkg.PendingInputRequest, error) {
	return h.pending, nil
}
func (terminalHandleStub) Signal(context.Context, terminalpkg.Actor, terminalpkg.Signal) error {
	return nil
}
func (terminalHandleStub) StartRecording(context.Context, terminalpkg.Actor) (terminalpkg.RecordingRef, error) {
	return terminalpkg.RecordingRef{}, nil
}
func (terminalHandleStub) StopRecording(context.Context, terminalpkg.Actor) (terminalpkg.RecordingRef, error) {
	return terminalpkg.RecordingRef{}, nil
}

type terminalSubscriptionStub struct {
	frames chan terminalpkg.Frame
	closed bool
}

func (s *terminalSubscriptionStub) Frames() <-chan terminalpkg.Frame { return s.frames }
func (*terminalSubscriptionStub) Err() error                         { return nil }
func (*terminalSubscriptionStub) Ack(int)                            {}
func (*terminalSubscriptionStub) Resize(uint16, uint16) error        { return nil }
func (s *terminalSubscriptionStub) Close() error {
	s.closed = true
	return nil
}

func terminalReadTransportFrame(t *testing.T, connection *websocket.Conn) terminalwire.Frame {
	t.Helper()
	messageType, encoded, err := connection.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if messageType != websocket.BinaryMessage {
		t.Fatalf("message type = %d, want binary", messageType)
	}
	frame, err := terminalwire.DecodeServer(encoded)
	if err != nil {
		t.Fatalf("DecodeServer() error = %v", err)
	}
	return frame
}
