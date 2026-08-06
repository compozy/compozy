package daytona

import (
	"context"
	"fmt"
	"io"
	"path"

	"github.com/compozy/compozy/internal/sandbox"
	"github.com/compozy/compozy/internal/subprocess"
)

var (
	_ sandbox.Launcher               = (*daytonaLauncher)(nil)
	_ sandbox.LaunchPreparer         = (*daytonaLauncher)(nil)
	_ sandbox.CommandRuntimeProvider = (*daytonaLauncher)(nil)
	_ sandbox.Handle                 = (*daytonaHandle)(nil)
)

type daytonaLauncher struct {
	transport transport
	sandbox   sandboxInfo
}

func (l *daytonaLauncher) Launch(
	ctx context.Context,
	spec sandbox.LaunchSpec,
) (sandbox.Handle, error) {
	if l == nil || l.transport == nil {
		return nil, fmt.Errorf("sandbox/daytona: launcher transport is required")
	}
	if spec.ResolvedExecutable != "" && !path.IsAbs(spec.ResolvedExecutable) {
		return nil, fmt.Errorf("sandbox/daytona: resolved launch executable must be absolute")
	}
	session, err := l.transport.Dial(ctx, l.sandbox, remoteLaunchCommand(spec))
	if err != nil {
		return nil, fmt.Errorf("sandbox/daytona: launch agent in sandbox: %w", err)
	}
	return &daytonaHandle{
		session: session,
		cwd:     spec.Cwd,
	}, nil
}

type daytonaHandle struct {
	session transportSession
	cwd     string
}

func (h *daytonaHandle) PID() int {
	return 0
}

func (h *daytonaHandle) Cwd() string {
	if h == nil {
		return ""
	}
	return h.cwd
}

func (h *daytonaHandle) Stdin() io.WriteCloser {
	if h == nil {
		return nil
	}
	return writeOnlySession{session: h.session}
}

func (h *daytonaHandle) Stdout() io.ReadCloser {
	if h == nil {
		return nil
	}
	return readOnlySession{session: h.session}
}

func (h *daytonaHandle) Stderr() string {
	if h == nil || h.session == nil {
		return ""
	}
	return h.session.Stderr()
}

func (h *daytonaHandle) ExitStatus() (subprocess.ExitStatus, bool) {
	if h == nil || h.session == nil {
		return subprocess.ExitStatus{}, false
	}
	exitCode, ok := h.session.ExitCode()
	if !ok {
		return subprocess.ExitStatus{}, false
	}
	return subprocess.ExitStatus{ExitCode: exitCode}, true
}

func (h *daytonaHandle) Done() <-chan struct{} {
	if h == nil || h.session == nil {
		done := make(chan struct{})
		close(done)
		return done
	}
	return h.session.Done()
}

func (h *daytonaHandle) Wait() error {
	if h == nil || h.session == nil {
		return nil
	}
	return h.session.Wait()
}

func (h *daytonaHandle) Stop(ctx context.Context) error {
	if h == nil || h.session == nil {
		return nil
	}
	return h.session.Stop(ctx)
}

type writeOnlySession struct {
	session transportSession
}

func (w writeOnlySession) Write(p []byte) (int, error) {
	return w.session.Write(p)
}

func (w writeOnlySession) Close() error {
	return w.session.CloseWrite()
}

type readOnlySession struct {
	session transportSession
}

func (r readOnlySession) Read(p []byte) (int, error) {
	return r.session.Read(p)
}

func (r readOnlySession) Close() error {
	return nil
}
