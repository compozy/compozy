package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestPrepareHostRuntimeStartsTransportsAndProbeReady(t *testing.T) {
	homeDir, err := os.MkdirTemp("/tmp", "daemon-host-*")
	if err != nil {
		t.Fatalf("MkdirTemp(/tmp) error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(homeDir)
	})
	t.Setenv("HOME", homeDir)

	paths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(homeDir, ".compozy"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	runCtx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()

	var runtime hostRuntime
	result, err := Start(context.Background(), StartOptions{
		HomePaths: paths,
		PID:       os.Getpid(),
		Version:   "test-run-host",
		HTTPPort:  EphemeralHTTPPort,
		ProcessAlive: func(pid int) bool {
			return pid == os.Getpid()
		},
		Healthy: ProbeReady,
		Prepare: func(startCtx context.Context, currentHost *Host) error {
			preparedRuntime, err := prepareHostRuntime(startCtx, runCtx, currentHost, func() {})
			if err != nil {
				return err
			}
			runtime = preparedRuntime
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if runtime.db == nil || runtime.httpServer == nil || runtime.udsServer == nil {
		t.Fatalf("prepareHostRuntime() returned incomplete runtime: %#v", runtime)
	}

	waitForCondition(t, 5*time.Second, "probe ready", func() bool {
		info := result.Host.Info()
		return info.State == ReadyStateReady && info.HTTPPort > 0 && ProbeReady(context.Background(), info) == nil
	})

	if err := closeHostRuntime(runtime, result.Host); err != nil {
		t.Fatalf("closeHostRuntime() error = %v", err)
	}
	if _, err := ReadInfo(paths.InfoPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ReadInfo(after close) error = %v, want os.ErrNotExist", err)
	}
}

func TestRunRejectsNilContext(t *testing.T) {
	t.Parallel()

	var nilCtx context.Context
	if err := Run(nilCtx, RunOptions{}); err == nil || !strings.Contains(err.Error(), "run context is required") {
		t.Fatalf("Run(nil) error = %v, want required context error", err)
	}
}

func TestDaemonHealthProblemUsesDetailMessage(t *testing.T) {
	t.Parallel()

	err := daemonHealthProblem(apicore.DaemonHealth{
		Ready: false,
		Details: []apicore.HealthDetail{
			{Code: "degraded", Message: "database warming"},
		},
	})
	if err == nil {
		t.Fatal("daemonHealthProblem() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "database warming") {
		t.Fatalf("daemonHealthProblem() = %v, want detailed message", err)
	}
}

func TestBuildHostHandlersWiresStopCallback(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{})

	stopped := false
	handlers := buildHostHandlers(&Host{}, hostPersistence{db: env.globalDB}, nil, func() {
		stopped = true
	})
	if handlers == nil || handlers.Daemon == nil || handlers.Workspaces == nil || handlers.Tasks == nil ||
		handlers.Reviews == nil || handlers.Sync == nil || handlers.Exec == nil {
		t.Fatalf("buildHostHandlers() returned incomplete handlers: %#v", handlers)
	}

	if err := handlers.Daemon.Stop(context.Background(), false); err != nil {
		t.Fatalf("handlers.Daemon.Stop() error = %v", err)
	}
	if !stopped {
		t.Fatal("handlers.Daemon.Stop() did not trigger stop callback")
	}
}
