package terminal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	terminalpty "github.com/compozy/compozy/internal/terminal/pty"
	"github.com/compozy/compozy/internal/toolruntime"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type fakePTY struct {
	mu      sync.Mutex
	procs   []*fakeProc
	starts  atomic.Int32
	started chan *fakeProc
}

func (p *fakePTY) Start(_ context.Context, spec ProcSpec) (Proc, error) {
	proc := newFakeProc(int(p.starts.Add(1)) + 1000)
	proc.spec = spec
	p.mu.Lock()
	p.procs = append(p.procs, proc)
	p.mu.Unlock()
	if p.started != nil {
		p.started <- proc
	}
	return proc, nil
}

func (p *fakePTY) latest() *fakeProc {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.procs) == 0 {
		return nil
	}
	return p.procs[len(p.procs)-1]
}

type fakeProc struct {
	reader         *io.PipeReader
	output         *io.PipeWriter
	done           chan struct{}
	completeOnce   sync.Once
	closeOnce      sync.Once
	mu             sync.Mutex
	input          bytes.Buffer
	resizes        []fakeResize
	exit           terminalpty.Exit
	completeErr    error
	spec           ProcSpec
	pid            int
	reads          atomic.Int32
	redactedWrites atomic.Int32
	echoWrites     atomic.Bool
	echoEnabled    bool
	visibilitySeen chan struct{}
	visibilityWait <-chan struct{}
	writeStarted   chan struct{}
	writeRelease   <-chan struct{}
	writeErr       error
}

type fakeResize struct {
	cols uint16
	rows uint16
}

func newFakeProc(pid int) *fakeProc {
	reader, output := io.Pipe()
	return &fakeProc{reader: reader, output: output, done: make(chan struct{}), pid: pid, echoEnabled: true}
}

func (p *fakeProc) Reader() io.Reader { return countingReader{Reader: p.reader, count: &p.reads} }

func (p *fakeProc) Write(input []byte) (int, error) {
	return p.writeInput(input, true)
}

func (p *fakeProc) writeInput(input []byte, allowEcho bool) (int, error) {
	p.mu.Lock()
	started := p.writeStarted
	p.writeStarted = nil
	release := p.writeRelease
	writeErr := p.writeErr
	p.mu.Unlock()
	if started != nil {
		close(started)
	}
	if release != nil {
		<-release
	}
	if writeErr != nil {
		return 0, writeErr
	}
	p.mu.Lock()
	written, err := p.input.Write(input)
	p.mu.Unlock()
	if err != nil || !allowEcho || !p.echoWrites.Load() {
		return written, err
	}
	return p.output.Write(input)
}

func (p *fakeProc) InputVisible() (bool, error) {
	p.mu.Lock()
	started := p.visibilitySeen
	p.visibilitySeen = nil
	release := p.visibilityWait
	visible := p.echoEnabled
	p.mu.Unlock()
	if started != nil {
		close(started)
	}
	if release != nil {
		<-release
	}
	return visible, nil
}

func (p *fakeProc) WriteRedacted(input []byte) (int, error) {
	p.redactedWrites.Add(1)
	return p.writeInput(input, false)
}

func (p *fakeProc) enableWriteEcho() { p.echoWrites.Store(true) }

func (p *fakeProc) blockWrites(started chan struct{}, release <-chan struct{}, err error) {
	p.mu.Lock()
	p.writeStarted = started
	p.writeRelease = release
	p.writeErr = err
	p.mu.Unlock()
}

func (p *fakeProc) blockInputVisibility(started chan struct{}, release <-chan struct{}) {
	p.mu.Lock()
	p.visibilitySeen = started
	p.visibilityWait = release
	p.mu.Unlock()
}

func (p *fakeProc) Resize(cols, rows uint16) error {
	p.mu.Lock()
	p.resizes = append(p.resizes, fakeResize{cols: cols, rows: rows})
	p.mu.Unlock()
	return nil
}

func (p *fakeProc) Wait(ctx context.Context) (terminalpty.Exit, error) {
	select {
	case <-ctx.Done():
		return terminalpty.Exit{}, ctx.Err()
	case <-p.done:
		p.mu.Lock()
		defer p.mu.Unlock()
		return p.exit, p.completeErr
	}
}

func (p *fakeProc) Kill(signal terminalpty.Signal) error {
	signalName := string(signal)
	p.complete(terminalpty.Exit{Cause: "signaled", Signal: &signalName})
	return nil
}

func (p *fakeProc) Close() error {
	var closeErr error
	p.closeOnce.Do(func() {
		closeErr = errors.Join(p.reader.Close(), p.output.Close())
	})
	return closeErr
}

func (p *fakeProc) PID() int             { return p.pid }
func (p *fakeProc) ProcessGroupID() int  { return p.pid }
func (p *fakeProc) StartedAt() time.Time { return time.Time{} }

func (p *fakeProc) emit(input []byte) error {
	_, err := p.output.Write(input)
	return err
}

func (p *fakeProc) complete(exit terminalpty.Exit) {
	p.completeOnce.Do(func() {
		p.mu.Lock()
		p.exit = exit
		p.mu.Unlock()
		p.completeErr = p.output.Close()
		close(p.done)
	})
}

func (p *fakeProc) inputString() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.input.String()
}

func (p *fakeProc) latestResize() (fakeResize, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.resizes) == 0 {
		return fakeResize{}, false
	}
	return p.resizes[len(p.resizes)-1], true
}

type countingReader struct {
	io.Reader
	count *atomic.Int32
}

func (r countingReader) Read(input []byte) (int, error) {
	r.count.Add(1)
	return r.Reader.Read(input)
}

type staticWorkspaceResolver struct {
	workspace      workspacepkg.ResolvedWorkspace
	profileCalls   atomic.Int32
	lastProfile    string
	lastProfileMux sync.Mutex
}

func (r *staticWorkspaceResolver) Resolve(_ context.Context, id string) (workspacepkg.ResolvedWorkspace, error) {
	resolved := r.workspace
	if id != "" {
		resolved.ID = id
		if strings.TrimSpace(resolved.WorkspaceID) == "" {
			resolved.WorkspaceID = id
		}
	}
	return resolved, nil
}

func (r *staticWorkspaceResolver) ResolveForProfile(
	_ context.Context,
	id string,
	profileName string,
) (workspacepkg.ResolvedWorkspace, error) {
	r.profileCalls.Add(1)
	r.lastProfileMux.Lock()
	r.lastProfile = profileName
	r.lastProfileMux.Unlock()
	resolved := r.workspace
	if id != "" {
		resolved.ID = id
		if strings.TrimSpace(resolved.WorkspaceID) == "" {
			resolved.WorkspaceID = id
		}
	}
	return resolved, nil
}

type profileNameMap map[string]string

func (m profileNameMap) ProfileName(_ context.Context, profileID string) (string, error) {
	name, ok := m[profileID]
	if !ok {
		return "", errors.New("profile name not found")
	}
	return name, nil
}

type fakeProfileGuard struct {
	mu     sync.Mutex
	errors map[string]error
}

type fakeTypingGrantAuthorizer struct {
	calls      atomic.Int32
	generation atomic.Uint64
	err        error
}

func (a *fakeTypingGrantAuthorizer) AuthorizeTerminalInput(_ context.Context, _ Actor, info Info) error {
	a.calls.Add(1)
	a.generation.Store(info.TypingGeneration)
	return a.err
}

func (g *fakeProfileGuard) EnsureAvailableID(_ context.Context, profileID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.errors[profileID]
}

type fakeCheckpoint struct {
	completed atomic.Bool
}

type cleanupProbeProc struct {
	Proc
	killErr      error
	closeErr     error
	waitErr      error
	waitCtx      chan cleanupContextObservation
	waitForBound bool
}

func (p *cleanupProbeProc) Kill(terminalpty.Signal) error { return p.killErr }

func (p *cleanupProbeProc) Close() error { return p.closeErr }

func (p *cleanupProbeProc) Wait(ctx context.Context) (terminalpty.Exit, error) {
	if p.waitForBound {
		<-ctx.Done()
	}
	p.waitCtx <- observeCleanupContext(ctx)
	return terminalpty.Exit{}, errors.Join(p.waitErr, ctx.Err())
}

type cleanupContextObservation struct {
	err         error
	value       any
	hasDeadline bool
}

type cleanupContextKey struct{}

type cancelWhenSubscriberInsertedContext struct {
	context.Context
	session *session
	cause   error
	done    chan struct{}
	once    sync.Once
}

func (c *cancelWhenSubscriberInsertedContext) Err() error {
	c.session.mu.RLock()
	inserted := len(c.session.subscribers) > 0
	c.session.mu.RUnlock()
	if inserted {
		c.once.Do(func() { close(c.done) })
		return c.cause
	}
	return nil
}

func (c *cancelWhenSubscriberInsertedContext) Done() <-chan struct{} { return c.done }

func observeCleanupContext(ctx context.Context) cleanupContextObservation {
	_, hasDeadline := ctx.Deadline()
	return cleanupContextObservation{err: ctx.Err(), value: ctx.Value(cleanupContextKey{}), hasDeadline: hasDeadline}
}

type cleanupProbeCheckpoint struct {
	completeErr error
	completeCtx chan cleanupContextObservation
}

func (*cleanupProbeCheckpoint) Checkpoint(context.Context, toolruntime.ProcessCheckpoint) error {
	return nil
}

func (c *cleanupProbeCheckpoint) Complete(ctx context.Context, _ toolruntime.ProcessCompletion) error {
	c.completeCtx <- observeCleanupContext(ctx)
	return c.completeErr
}

type fakeRecordingJournal struct {
	mu        sync.Mutex
	contents  []byte
	ref       RecordingRef
	called    chan struct{}
	calledOne sync.Once
	release   <-chan struct{}
	rows      []CommandRow
}

func (j *fakeRecordingJournal) Record(_ context.Context, _ string, row CommandRow) error {
	j.mu.Lock()
	j.rows = append(j.rows, row)
	j.mu.Unlock()
	return nil
}

func (j *fakeRecordingJournal) Query(context.Context, string, store.ReadScope, Query) (*Page, error) {
	return &Page{Entries: []CommandRow{}}, nil
}

func (j *fakeRecordingJournal) LinkRecording(context.Context, string, ID, RecordingRef) error {
	return nil
}

func (j *fakeRecordingJournal) Recording(
	context.Context,
	string,
	store.ReadScope,
	string,
) (*RecordingRef, io.ReadCloser, error) {
	return nil, nil, errors.New("recording not found")
}

func (j *fakeRecordingJournal) Artifact(context.Context, string, store.ReadScope, string) (io.ReadCloser, error) {
	return nil, errors.New("artifact not found")
}

func (j *fakeRecordingJournal) PersistRecording(
	_ context.Context,
	_ string,
	_ ID,
	ref RecordingRef,
	contents []byte,
) (RecordingRef, error) {
	if j.called != nil {
		j.calledOne.Do(func() { close(j.called) })
	}
	if j.release != nil {
		<-j.release
	}
	digest := sha256.Sum256(contents)
	ref.Digest = hex.EncodeToString(digest[:])
	ref.Path = "/recordings/" + ref.Digest + ".cast"
	ref.Bytes = int64(len(contents))
	j.mu.Lock()
	j.contents = append([]byte(nil), contents...)
	j.ref = ref
	j.mu.Unlock()
	return ref, nil
}

func (j *fakeRecordingJournal) snapshot() (RecordingRef, []byte) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.ref, append([]byte(nil), j.contents...)
}

func (*fakeCheckpoint) Checkpoint(context.Context, toolruntime.ProcessCheckpoint) error { return nil }

func (c *fakeCheckpoint) Complete(context.Context, toolruntime.ProcessCompletion) error {
	c.completed.Store(true)
	return nil
}

func newTestManager(
	t *testing.T,
	settings Settings,
	options ...Option,
) (*Service, *fakePTY, string) {
	t.Helper()
	root := t.TempDir()
	starter := &fakePTY{started: make(chan *fakeProc, 64)}
	resolver := &staticWorkspaceResolver{workspace: workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{ID: "workspace-a", RootDir: root}, WorkspaceID: "workspace-a",
	}}
	base := []Option{
		WithPTY(starter),
		WithWorkspaceResolver(resolver),
		WithSettingsProvider(func(context.Context, string, string) (Settings, error) { return settings, nil }),
	}
	base = append(base, options...)
	manager, err := NewManager(base...)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := manager.Shutdown(ctx); err != nil {
			t.Errorf("Shutdown() cleanup error = %v", err)
		}
	})
	return manager, starter, root
}

func openTestTerminal(t *testing.T, manager *Service, workspaceID, profileID string) Handle {
	t.Helper()
	handle, err := manager.Open(context.Background(), OpenRequest{
		WS: workspaceID, Shell: "sh", Actor: Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: profileID},
		Capabilities: Capabilities{Interactive: true}, Cols: 80, Rows: 24,
	})
	if err != nil {
		t.Fatalf("Open(%s,%s) error = %v", workspaceID, profileID, err)
	}
	return handle
}
