package daemon

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestPrepareHostRuntimeStartsTransportsAndProbeReady(t *testing.T) {
	homeDir, err := os.MkdirTemp("", "cmp-home-")
	if err != nil {
		t.Fatalf("MkdirTemp() error = %v", err)
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

	rootRequest, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"http://127.0.0.1:"+strconv.Itoa(result.Host.Info().HTTPPort)+"/",
		http.NoBody,
	)
	if err != nil {
		t.Fatalf("NewRequestWithContext(/) error = %v", err)
	}
	rootResponse, err := (&http.Client{Timeout: time.Second}).Do(rootRequest)
	if err != nil {
		t.Fatalf("GET / error = %v", err)
	}
	rootBody, err := io.ReadAll(rootResponse.Body)
	if err != nil {
		_ = rootResponse.Body.Close()
		t.Fatalf("ReadAll(/) error = %v", err)
	}
	if err := rootResponse.Body.Close(); err != nil {
		t.Fatalf("Close(/) body error = %v", err)
	}
	if rootResponse.StatusCode != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", rootResponse.StatusCode, http.StatusOK)
	}
	if !bytes.Contains(rootBody, []byte(`<div id="app"></div>`)) {
		t.Fatalf("GET / body = %q, want SPA shell", rootBody)
	}

	if err := closeHostRuntime(context.Background(), runtime, result.Host); err != nil {
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
