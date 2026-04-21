package daemon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	apiclient "github.com/compozy/compozy/internal/api/client"
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

func TestStartRecoversAfterKilledDaemonLeavesStaleArtifacts(t *testing.T) {
	paths := mustHomePaths(t)
	helper := startDaemonHelperProcess(t, paths)

	waitForHealthyDaemon(t, paths, helper.Process.Pid)
	killDaemonHelperProcess(t, helper, syscall.SIGKILL)

	if err := os.WriteFile(paths.SocketPath, []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale socket marker: %v", err)
	}

	result, err := Start(context.Background(), StartOptions{
		HomePaths: paths,
		Version:   "recovery-test",
	})
	if err != nil {
		t.Fatalf("Start(recovery) error = %v", err)
	}
	if result.Outcome != StartOutcomeStarted {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, StartOutcomeStarted)
	}
	defer func() {
		_ = result.Host.Close(context.Background())
	}()

	if result.Info.PID == helper.Process.Pid {
		t.Fatalf("Info.PID = %d, want a new pid after crash recovery", result.Info.PID)
	}

	status, err := QueryStatus(context.Background(), paths, ProbeOptions{})
	if err != nil {
		t.Fatalf("QueryStatus() error = %v", err)
	}
	if !status.Healthy || status.State != ReadyStateReady {
		t.Fatalf("status = %#v, want ready and healthy", status)
	}
	if status.Info == nil || status.Info.PID != result.Info.PID {
		t.Fatalf("status info = %#v, want pid %d", status.Info, result.Info.PID)
	}

	if _, err := os.Stat(paths.SocketPath); !os.IsNotExist(err) {
		t.Fatalf("expected stale socket to be removed, stat err = %v", err)
	}
}

func TestStartUsesSameHomeScopedDaemonAcrossWorkspaces(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	paths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(homeDir, ".compozy"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	workspaceA := filepath.Join(t.TempDir(), "workspace-a")
	workspaceB := filepath.Join(t.TempDir(), "workspace-b")
	nestedA := filepath.Join(workspaceA, "pkg", "feature-a")
	nestedB := filepath.Join(workspaceB, "pkg", "feature-b")
	for _, dir := range []string{nestedA, nestedB} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir nested dir %q: %v", dir, err)
		}
	}

	helper := startDaemonHelperProcess(t, paths)
	defer stopDaemonHelperProcess(t, helper)

	waitForHealthyDaemon(t, paths, helper.Process.Pid)

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(nestedB); err != nil {
		t.Fatalf("Chdir(%s) error = %v", nestedB, err)
	}
	defer func() {
		_ = os.Chdir(originalWD)
	}()

	result, err := Start(context.Background(), StartOptions{
		Version: "workspace-b",
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if result.Outcome != StartOutcomeAlreadyRunning {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, StartOutcomeAlreadyRunning)
	}
	if result.Info.PID != helper.Process.Pid {
		t.Fatalf("Info.PID = %d, want %d", result.Info.PID, helper.Process.Pid)
	}
	if result.Paths.HomeDir != paths.HomeDir {
		t.Fatalf("HomeDir = %q, want %q", result.Paths.HomeDir, paths.HomeDir)
	}

	for _, workspaceRoot := range []string{workspaceA, workspaceB} {
		workspaceDaemonDir := filepath.Join(workspaceRoot, ".compozy", "daemon")
		if _, err := os.Stat(workspaceDaemonDir); !os.IsNotExist(err) {
			t.Fatalf("expected no workspace-scoped daemon dir at %q, stat err = %v", workspaceDaemonDir, err)
		}
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

func TestManagedDaemonHelperProcess(t *testing.T) {
	if os.Getenv("COMPOZY_MANAGED_DAEMON_HELPER") != "1" {
		t.Skip("managed helper process")
	}

	homeRoot := os.Getenv("COMPOZY_DAEMON_HOME")
	if strings.TrimSpace(homeRoot) == "" {
		t.Fatal("COMPOZY_DAEMON_HOME is required")
	}
	t.Setenv("HOME", filepath.Dir(homeRoot))

	mode := RunMode(os.Getenv("COMPOZY_MANAGED_DAEMON_MODE"))
	if resolveRunMode(mode) == RunModeForeground {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := Run(ctx, RunOptions{
			Version:  "managed-helper",
			HTTPPort: EphemeralHTTPPort,
			Mode:     RunModeForeground,
		}); err != nil {
			t.Fatalf("Run(foreground) error = %v", err)
		}
		return
	}

	if err := Run(context.Background(), RunOptions{
		Version:  "managed-helper",
		HTTPPort: EphemeralHTTPPort,
		Mode:     RunModeDetached,
	}); err != nil {
		t.Fatalf("Run(detached) error = %v", err)
	}
}

func TestManagedDaemonStopEndpointShutsDownAndRemovesSocket(t *testing.T) {
	for _, force := range []bool{false, true} {
		t.Run("Should stop and remove socket when force="+strconv.FormatBool(force), func(t *testing.T) {
			paths := mustHomePaths(t)
			helper, output := startManagedDaemonHelperProcess(t, paths, RunModeDetached)

			waitForManagedDaemonReady(t, paths, helper.Process.Pid, output)

			status, err := QueryStatus(context.Background(), paths, ProbeOptions{Healthy: ProbeReady})
			if err != nil {
				t.Fatalf("QueryStatus() error = %v", err)
			}
			if status.Info == nil {
				t.Fatal("status.Info = nil, want running daemon info")
			}

			client, err := apiclient.New(apiclient.Target{
				SocketPath: status.Info.SocketPath,
				HTTPPort:   status.Info.HTTPPort,
			})
			if err != nil {
				t.Fatalf("apiclient.New() error = %v", err)
			}

			start := time.Now()
			if err := client.StopDaemon(context.Background(), force); err != nil {
				t.Fatalf("StopDaemon(force=%t) error = %v", force, err)
			}
			waitForDaemonProcessExit(t, helper)
			elapsed := time.Since(start)
			if elapsed > defaultShutdownDrainTimeout {
				t.Fatalf("StopDaemon(force=%t) elapsed = %v, want <= %v", force, elapsed, defaultShutdownDrainTimeout)
			}

			waitForDaemonState(t, paths, ReadyStateStopped)
			if _, err := os.Stat(paths.SocketPath); !os.IsNotExist(err) {
				t.Fatalf("expected socket path to be removed after stop, stat err = %v", err)
			}
			waitForLogContains(t, paths.LogFile, `"msg":"daemon stop accepted"`)
			waitForLogContains(t, paths.LogFile, `"force":`+strconv.FormatBool(force))
		})
	}
}

func TestManagedDaemonRunModesControlLogging(t *testing.T) {
	t.Run("Should mirror logs to stderr in foreground mode", func(t *testing.T) {
		paths := mustHomePaths(t)
		helper, output := startManagedDaemonHelperProcess(t, paths, RunModeForeground)

		waitForManagedDaemonReady(t, paths, helper.Process.Pid, output)
		waitForLogContains(t, paths.LogFile, `"msg":"daemon started"`)
		waitForStderrContains(t, &output.stderr, `"msg":"daemon started"`)

		stopDaemonHelperProcess(t, helper)
		waitForLogContains(t, paths.LogFile, `"mode":"foreground"`)
	})

	t.Run("Should write logs only to file in detached mode", func(t *testing.T) {
		paths := mustHomePaths(t)
		helper, output := startManagedDaemonHelperProcess(t, paths, RunModeDetached)

		waitForManagedDaemonReady(t, paths, helper.Process.Pid, output)
		waitForLogContains(t, paths.LogFile, `"msg":"daemon started"`)
		stopDaemonHelperProcess(t, helper)

		if strings.Contains(output.stderr.String(), `"msg":"daemon started"`) {
			t.Fatalf("expected detached mode to avoid stderr mirroring, got %q", output.stderr.String())
		}
		waitForLogContains(t, paths.LogFile, `"mode":"detached"`)
	})
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

func startManagedDaemonHelperProcess(
	t *testing.T,
	paths compozyconfig.HomePaths,
	mode RunMode,
) (*exec.Cmd, *managedHelperOutput) {
	t.Helper()

	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run", "^TestManagedDaemonHelperProcess$")
	cmd.Env = append(os.Environ(),
		"COMPOZY_MANAGED_DAEMON_HELPER=1",
		"COMPOZY_MANAGED_DAEMON_MODE="+string(resolveRunMode(mode)),
		"COMPOZY_DAEMON_HOME="+paths.HomeDir,
	)
	output := &managedHelperOutput{}
	cmd.Stdout = &output.stdout
	cmd.Stderr = &output.stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start managed helper process: %v", err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
			if err := killTestProcess(cmd); err != nil {
				t.Errorf("kill managed helper process during cleanup: %v", err)
				return
			}
			if err := waitTestProcess(cmd); err != nil {
				t.Errorf("wait managed helper process during cleanup: %v", err)
			}
		}
	})
	return cmd, output
}

func stopDaemonHelperProcess(t *testing.T, cmd *exec.Cmd) {
	t.Helper()

	if cmd == nil || cmd.Process == nil {
		return
	}
	killDaemonHelperProcess(t, cmd, syscall.SIGTERM)
}

func killDaemonHelperProcess(t *testing.T, cmd *exec.Cmd, sig syscall.Signal) {
	t.Helper()

	if cmd == nil || cmd.Process == nil {
		return
	}
	if err := signalTestProcess(cmd, sig); err != nil {
		t.Fatalf("signal helper process with %s: %v", sig, err)
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil && sig != syscall.SIGKILL {
			t.Fatalf("wait helper process: %v", err)
		}
	case <-time.After(10 * time.Second):
		if err := killTestProcess(cmd); err != nil {
			t.Fatalf("timed out waiting for helper process shutdown; Kill() error = %v", err)
		}
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

func waitForManagedDaemonReady(
	t *testing.T,
	paths compozyconfig.HomePaths,
	wantPID int,
	output *managedHelperOutput,
) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		status, err := QueryStatus(context.Background(), paths, ProbeOptions{Healthy: ProbeReady})
		if err == nil && status.Healthy && status.Info != nil && status.Info.PID == wantPID {
			return
		}
		if time.Now().After(deadline) {
			break
		}
		<-ticker.C
	}

	infoSummary := describeDaemonInfoForFailure(paths.InfoPath)
	logSummary := readDiagnosticFileForFailure(paths.LogFile)
	if output == nil {
		t.Fatalf(
			"managed daemon did not become transport-healthy for pid %d within timeout\ninfo=%s\nlog=%s",
			wantPID,
			infoSummary,
			logSummary,
		)
	}
	t.Fatalf(
		"managed daemon did not become transport-healthy for pid %d within timeout\ninfo=%s\nstdout=%s\nstderr=%s\nlog=%s",
		wantPID,
		infoSummary,
		output.stdout.String(),
		output.stderr.String(),
		logSummary,
	)
}

func waitForDaemonProcessExit(t *testing.T, cmd *exec.Cmd) {
	t.Helper()

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("wait daemon process: %v", err)
		}
	case <-time.After(10 * time.Second):
		if err := killTestProcess(cmd); err != nil {
			t.Fatalf("timed out waiting for daemon process exit; Kill() error = %v", err)
		}
		t.Fatal("timed out waiting for daemon process exit")
	}
}

func waitForDaemonState(t *testing.T, paths compozyconfig.HomePaths, want ReadyState) {
	t.Helper()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case <-ticker.C:
			status, err := QueryStatus(context.Background(), paths, ProbeOptions{})
			if err == nil && status.State == want {
				return
			}
		case <-timeout.C:
			t.Fatalf("daemon did not reach state %q within timeout", want)
		}
	}
}

func waitForLogContains(t *testing.T, path string, pattern string) {
	t.Helper()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()

	for {
		data, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(data), pattern) {
			return
		}
		select {
		case <-ticker.C:
		case <-timeout.C:
			t.Fatalf(
				"log %q did not contain %q within timeout\n%s",
				path,
				pattern,
				readDiagnosticFileForFailure(path),
			)
		}
	}
}

func signalTestProcess(cmd *exec.Cmd, sig os.Signal) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if err := cmd.Process.Signal(sig); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

func killTestProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

func waitTestProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	_, err := cmd.Process.Wait()
	if err != nil && !errors.Is(err, os.ErrProcessDone) && !errors.Is(err, syscall.ECHILD) {
		return err
	}
	return nil
}

func describeDaemonInfoForFailure(path string) string {
	info, err := ReadInfo(path)
	if err != nil {
		return fmt.Sprintf("read daemon info %q: %v", path, err)
	}
	return fmt.Sprintf("%#v", info)
}

func readDiagnosticFileForFailure(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("read file %q: %v", path, err)
	}
	return string(data)
}

func waitForStderrContains(t *testing.T, stderr *synchronizedBuffer, pattern string) {
	t.Helper()

	if stderr == nil {
		t.Fatalf("stderr buffer = nil, want output containing %q", pattern)
	}

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()

	for {
		if strings.Contains(stderr.String(), pattern) {
			return
		}
		select {
		case <-ticker.C:
		case <-timeout.C:
			t.Fatalf("stderr did not contain %q within timeout\n%s", pattern, stderr.String())
		}
	}
}

type managedHelperOutput struct {
	stdout synchronizedBuffer
	stderr synchronizedBuffer
}

type synchronizedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *synchronizedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
