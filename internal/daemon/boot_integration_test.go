package daemon

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestStartTwiceLeavesOneHealthySingletonInstance(t *testing.T) {
	paths := mustHomePaths(t)
	helper := startDaemonHelperProcess(t, paths)
	defer stopDaemonHelperProcess(t, helper)

	waitForHealthyDaemon(t, paths, helper.Process.Pid)

	result, err := Start(context.Background(), StartOptions{
		HomePaths: paths,
		Version:   "integration-test",
	})
	if err != nil {
		t.Fatalf("Start(second) error = %v", err)
	}
	if result.Outcome != StartOutcomeAlreadyRunning {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, StartOutcomeAlreadyRunning)
	}
	if result.Info.PID != helper.Process.Pid {
		t.Fatalf("Info.PID = %d, want %d", result.Info.PID, helper.Process.Pid)
	}

	status, err := QueryStatus(context.Background(), paths, ProbeOptions{})
	if err != nil {
		t.Fatalf("QueryStatus() error = %v", err)
	}
	if !status.Healthy {
		t.Fatal("Healthy = false, want true")
	}
	if status.Info == nil || status.Info.PID != helper.Process.Pid {
		t.Fatalf("status info = %#v, want pid %d", status.Info, helper.Process.Pid)
	}
}

func TestStartUsesHomeScopedLayoutFromWorkspaceSubdirectory(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	nestedDir := filepath.Join(workspaceRoot, "pkg", "feature", "subdir")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested dir: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(nestedDir); err != nil {
		t.Fatalf("Chdir(%s) error = %v", nestedDir, err)
	}
	defer func() {
		_ = os.Chdir(originalWD)
	}()

	result, err := Start(context.Background(), StartOptions{
		Version: "cwd-test",
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		_ = result.Host.Close(context.Background())
	}()

	wantPaths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(homeDir, ".compozy"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	if result.Paths.HomeDir != wantPaths.HomeDir {
		t.Fatalf("HomeDir = %q, want %q", result.Paths.HomeDir, wantPaths.HomeDir)
	}
	if result.Paths.DaemonDir != wantPaths.DaemonDir {
		t.Fatalf("DaemonDir = %q, want %q", result.Paths.DaemonDir, wantPaths.DaemonDir)
	}
	if result.Paths.HomeDir == filepath.Join(workspaceRoot, ".compozy") {
		t.Fatalf("HomeDir should not be workspace-scoped: %q", result.Paths.HomeDir)
	}

	status, err := QueryStatus(context.Background(), compozyconfig.HomePaths{}, ProbeOptions{})
	if err != nil {
		t.Fatalf("QueryStatus() error = %v", err)
	}
	if status.State != ReadyStateReady || !status.Healthy {
		t.Fatalf("status = %#v, want ready and healthy", status)
	}
	if status.Info == nil || status.Info.SocketPath != wantPaths.SocketPath {
		t.Fatalf("status info = %#v, want socket %q", status.Info, wantPaths.SocketPath)
	}
}

func TestDaemonHelperProcess(t *testing.T) {
	if os.Getenv("COMPOZY_DAEMON_HELPER") != "1" {
		t.Skip("helper process")
	}

	homeRoot := os.Getenv("COMPOZY_DAEMON_HOME")
	paths, err := compozyconfig.ResolveHomePathsFrom(homeRoot)
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	result, err := Start(ctx, StartOptions{
		HomePaths: paths,
		Version:   "helper",
	})
	if err != nil {
		stop()
		t.Fatalf("Start() error = %v", err)
	}
	if result.Outcome == StartOutcomeAlreadyRunning {
		stop()
		return
	}
	defer func() {
		_ = result.Host.Close(context.Background())
	}()
	defer stop()

	<-ctx.Done()
}

func startDaemonHelperProcess(t *testing.T, paths compozyconfig.HomePaths) *exec.Cmd {
	t.Helper()

	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run", "^TestDaemonHelperProcess$")
	cmd.Env = append(os.Environ(),
		"COMPOZY_DAEMON_HELPER=1",
		"COMPOZY_DAEMON_HOME="+paths.HomeDir,
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper process: %v", err)
	}
	return cmd
}

func stopDaemonHelperProcess(t *testing.T, cmd *exec.Cmd) {
	t.Helper()

	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("wait helper process: %v", err)
		}
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("timed out waiting for helper process shutdown")
	}
}

func waitForHealthyDaemon(t *testing.T, paths compozyconfig.HomePaths, wantPID int) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		status, err := QueryStatus(context.Background(), paths, ProbeOptions{})
		if err == nil && status.Healthy && status.Info != nil && status.Info.PID == wantPID {
			return
		}
		if time.Now().After(deadline) {
			break
		}
		<-ticker.C
	}
	t.Fatalf("daemon did not become healthy for pid %d within timeout", wantPID)
}
