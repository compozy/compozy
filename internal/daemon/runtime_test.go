package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/httpapi"
	"github.com/compozy/compozy/internal/api/udsapi"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/journal"
	"github.com/compozy/compozy/internal/store/globaldb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
)

type closeHostRuntimeContextKey string

type capturingRunScope struct {
	closeCtx context.Context
}

func (*capturingRunScope) RunArtifacts() model.RunArtifacts {
	return model.RunArtifacts{}
}

func (*capturingRunScope) RunJournal() *journal.Journal {
	return nil
}

func (*capturingRunScope) RunEventBus() *eventspkg.Bus[eventspkg.Event] {
	return nil
}

func (*capturingRunScope) RunManager() model.RuntimeManager {
	return nil
}

func (c *capturingRunScope) Close(ctx context.Context) error {
	c.closeCtx = ctx
	return nil
}

func TestCloseHostRuntimeUsesBoundedContexts(t *testing.T) {
	originalShutdownRunManager := shutdownRunManager
	originalShutdownHTTPServer := shutdownHTTPServer
	originalShutdownUDSServer := shutdownUDSServer
	originalCloseHostGlobalDB := closeHostGlobalDB
	originalCloseDaemonHost := closeDaemonHost
	t.Cleanup(func() {
		shutdownRunManager = originalShutdownRunManager
		shutdownHTTPServer = originalShutdownHTTPServer
		shutdownUDSServer = originalShutdownUDSServer
		closeHostGlobalDB = originalCloseHostGlobalDB
		closeDaemonHost = originalCloseDaemonHost
	})

	var captured []context.Context
	record := func(ctx context.Context) error {
		captured = append(captured, ctx)
		return nil
	}
	shutdownRunManager = func(ctx context.Context, _ *RunManager) error { return record(ctx) }
	shutdownHTTPServer = func(ctx context.Context, _ *httpapi.Server) error { return record(ctx) }
	shutdownUDSServer = func(ctx context.Context, _ *udsapi.Server) error { return record(ctx) }
	closeHostGlobalDB = func(ctx context.Context, _ *globaldb.GlobalDB) error { return record(ctx) }
	closeDaemonHost = func(ctx context.Context, _ *Host) error { return record(ctx) }

	parentCtx, cancel := context.WithCancel(
		context.WithValue(context.Background(), closeHostRuntimeContextKey("scope"), "runtime-shutdown"),
	)
	cancel()

	runtime := hostRuntime{
		runManager:      &RunManager{},
		httpServer:      &httpapi.Server{},
		udsServer:       &udsapi.Server{},
		db:              &globaldb.GlobalDB{},
		shutdownTimeout: 250 * time.Millisecond,
	}
	if err := closeHostRuntime(parentCtx, runtime, &Host{}); err != nil {
		t.Fatalf("closeHostRuntime() error = %v", err)
	}
	if got := len(captured); got != 5 {
		t.Fatalf("captured shutdown contexts = %d, want 5", got)
	}

	for index, ctx := range captured {
		if ctx == nil {
			t.Fatalf("captured context %d = nil", index)
		}
		if ctx.Err() != nil {
			t.Fatalf("captured context %d err = %v, want nil", index, ctx.Err())
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Fatalf("captured context %d missing deadline", index)
		}
		if got := ctx.Value(closeHostRuntimeContextKey("scope")); got != "runtime-shutdown" {
			t.Fatalf("captured context %d value = %#v, want runtime-shutdown", index, got)
		}
	}
}

func TestCloseRunScopeUsesBoundedContext(t *testing.T) {
	parentCtx, cancel := context.WithCancel(
		context.WithValue(context.Background(), closeHostRuntimeContextKey("scope"), "run-scope-close"),
	)
	cancel()

	scope := &capturingRunScope{}
	if err := closeRunScope(parentCtx, scope, 250*time.Millisecond); err != nil {
		t.Fatalf("closeRunScope() error = %v", err)
	}
	if scope.closeCtx == nil {
		t.Fatal("expected closeRunScope to invoke RunScope.Close")
	}
	if scope.closeCtx.Err() != nil {
		t.Fatalf("close scope context err = %v, want nil", scope.closeCtx.Err())
	}
	if _, ok := scope.closeCtx.Deadline(); !ok {
		t.Fatal("expected close scope context deadline")
	}
	if got := scope.closeCtx.Value(closeHostRuntimeContextKey("scope")); got != "run-scope-close" {
		t.Fatalf("close scope context value = %#v, want run-scope-close", got)
	}
}

func TestDaemonRunSignalContextDetachedIgnoresCallerCancellation(t *testing.T) {
	parentCtx, cancelParent := context.WithCancel(context.Background())
	cancelParent()

	detachedCtx, stop, err := daemonRunSignalContext(parentCtx, RunModeDetached)
	if err != nil {
		t.Fatalf("daemonRunSignalContext(detached) error = %v", err)
	}
	defer stop()

	if detachedCtx.Err() != nil {
		t.Fatalf("detached signal context err = %v, want nil after parent cancellation", detachedCtx.Err())
	}

	foregroundCtx, stopForeground, err := daemonRunSignalContext(parentCtx, RunModeForeground)
	if err != nil {
		t.Fatalf("daemonRunSignalContext(foreground) error = %v", err)
	}
	defer stopForeground()

	if foregroundCtx.Err() == nil {
		t.Fatal("expected foreground signal context to preserve caller cancellation")
	}
}
