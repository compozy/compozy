package daemon

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/store/globaldb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/gin-gonic/gin"
)

func TestStopDaemonHTTPReturnsConflictThenForceCancelsActiveRun(t *testing.T) {
	gin.SetMode(gin.TestMode)

	started := make(chan string, 1)
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			started <- cfg.RunID
			<-ctx.Done()
			return ctx.Err()
		},
	})

	var stopCalls atomic.Int64
	service := NewService(ServiceConfig{
		GlobalDB:    env.globalDB,
		RunManager:  env.manager,
		RequestStop: func(context.Context) error { stopCalls.Add(1); return nil },
	})

	engine := gin.New()
	apicore.RegisterRoutes(engine, apicore.NewHandlers(&apicore.HandlerConfig{
		TransportName: "http",
		Daemon:        service,
	}))

	run := env.startTaskRun(t, "task-run-stop-http", nil)
	waitForString(t, started, run.RunID)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/daemon/stop", http.NoBody)
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("POST /api/daemon/stop status = %d, want 409", recorder.Code)
	}
	if stopCalls.Load() != 0 {
		t.Fatalf("stop callback calls = %d, want 0 after conflict", stopCalls.Load())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/daemon/stop?force=true",
		http.NoBody,
	)
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("POST /api/daemon/stop?force=true status = %d, want 202", recorder.Code)
	}
	if stopCalls.Load() != 1 {
		t.Fatalf("stop callback calls = %d, want 1 after forced stop", stopCalls.Load())
	}

	terminal := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCancelled
	})
	if terminal.EndedAt == nil {
		t.Fatal("EndedAt = nil, want terminal timestamp")
	}
	if terminal.ErrorText == "" {
		t.Fatal("ErrorText = empty, want cancellation reason")
	}

	lastEvent := env.lastRunEvent(t, run.RunID)
	if lastEvent == nil || lastEvent.Kind != eventspkg.EventKindRunCancelled {
		t.Fatalf("last event = %#v, want run.cancelled", lastEvent)
	}
}

func TestRunManagerShutdownHonorsDrainTimeoutAndKeepsTerminalState(t *testing.T) {
	shutdownEntered := make(chan struct{}, 1)
	runtimeManager := &blockingShutdownRuntimeManager{shutdownEntered: shutdownEntered}
	started := make(chan string, 1)

	env := newRunManagerTestEnv(t, runManagerTestDeps{
		shutdownDrainTimeout: 150 * time.Millisecond,
		openRunScope:         newTestOpenRunScope(runtimeManager),
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			started <- cfg.RunID
			<-ctx.Done()
			return ctx.Err()
		},
	})

	run := env.startTaskRun(t, "task-run-drain-timeout", nil)
	waitForString(t, started, run.RunID)

	start := time.Now()
	if err := env.manager.Shutdown(context.Background(), true); err != nil {
		t.Fatalf("Shutdown(force) error = %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 120*time.Millisecond {
		t.Fatalf("Shutdown(force) elapsed = %v, want wait near configured drain timeout", elapsed)
	}
	if elapsed > time.Second {
		t.Fatalf("Shutdown(force) elapsed = %v, want bounded return", elapsed)
	}

	select {
	case <-shutdownEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("runtime shutdown was not entered")
	}

	terminal := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCancelled
	})
	if terminal.EndedAt == nil {
		t.Fatal("EndedAt = nil, want terminal timestamp")
	}
	lastEvent := env.lastRunEvent(t, run.RunID)
	if lastEvent == nil || lastEvent.Kind != eventspkg.EventKindRunCancelled {
		t.Fatalf("last event = %#v, want run.cancelled", lastEvent)
	}
}

type blockingShutdownRuntimeManager struct {
	shutdownEntered chan<- struct{}
}

func (*blockingShutdownRuntimeManager) Start(context.Context) error {
	return nil
}

func (*blockingShutdownRuntimeManager) DispatchMutableHook(
	_ context.Context,
	_ string,
	payload any,
) (any, error) {
	return payload, nil
}

func (*blockingShutdownRuntimeManager) DispatchObserverHook(context.Context, string, any) {}

func (m *blockingShutdownRuntimeManager) Shutdown(ctx context.Context) error {
	select {
	case m.shutdownEntered <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return ctx.Err()
}

func TestRunManagerShutdownWithoutForceReturnsConflictProblem(t *testing.T) {
	started := make(chan string, 1)
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			started <- cfg.RunID
			<-ctx.Done()
			return ctx.Err()
		},
	})

	run := env.startTaskRun(t, "task-run-stop-conflict", nil)
	waitForString(t, started, run.RunID)

	err := env.manager.Shutdown(context.Background(), false)
	var problem *apicore.Problem
	if !errors.As(err, &problem) {
		t.Fatalf("Shutdown(false) error = %T %v, want *core.Problem", err, err)
	}
	if problem.Status != http.StatusConflict {
		t.Fatalf("problem status = %d, want 409", problem.Status)
	}

	if forceErr := env.manager.Shutdown(context.Background(), true); forceErr != nil {
		t.Fatalf("Shutdown(force cleanup) error = %v", forceErr)
	}
}
