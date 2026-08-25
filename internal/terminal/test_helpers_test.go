package terminal

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	terminalpty "github.com/compozy/compozy/internal/terminal/pty"
	"github.com/compozy/compozy/internal/toolruntime"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type fakePTY struct {
	mu     sync.Mutex
	procs  []*fakeProc
	starts atomic.Int32
}

func (p *fakePTY) Start(_ context.Context, spec ProcSpec) (Proc, error) {
	proc := newFakeProc(int(p.starts.Add(1)) + 1000)
	proc.spec = spec
	p.mu.Lock()
	p.procs = append(p.procs, proc)
	p.mu.Unlock()
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
	reader       *io.PipeReader
	output       *io.PipeWriter
	done         chan struct{}
	completeOnce sync.Once
	closeOnce    sync.Once
	mu           sync.Mutex
	input        bytes.Buffer
	resizes      []fakeResize
	exit         terminalpty.Exit
	completeErr  error
	spec         ProcSpec
	pid          int
	reads        atomic.Int32
}

type fakeResize struct {
	cols uint16
	rows uint16
}

func newFakeProc(pid int) *fakeProc {
	reader, output := io.Pipe()
	return &fakeProc{reader: reader, output: output, done: make(chan struct{}), pid: pid}
}

func (p *fakeProc) Reader() io.Reader { return countingReader{Reader: p.reader, count: &p.reads} }

func (p *fakeProc) Write(input []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.input.Write(input)
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

func (p *fakeProc) PID() int            { return p.pid }
func (p *fakeProc) ProcessGroupID() int { return p.pid }

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
		resolved.WorkspaceID = id
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
		resolved.WorkspaceID = id
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

func (g *fakeProfileGuard) EnsureAvailableID(_ context.Context, profileID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.errors[profileID]
}

type fakeCheckpoint struct {
	completed atomic.Bool
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
	starter := &fakePTY{}
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
