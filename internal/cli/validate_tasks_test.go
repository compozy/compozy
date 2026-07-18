package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/compozy/compozy/internal/daemon"
)

const (
	validateTasksGuardianModeEnv          = "COMPOZY_TEST_ARTIFACT_GUARDIAN"
	validateTasksGuardianBuildDirEnv      = "COMPOZY_TEST_ARTIFACT_GUARDIAN_BUILD_DIR"
	validateTasksGuardianErrorPathEnv     = "COMPOZY_TEST_ARTIFACT_GUARDIAN_ERROR_PATH"
	validateTasksGuardianOwnerHelperEnv   = "COMPOZY_TEST_ARTIFACT_GUARDIAN_OWNER_HELPER"
	validateTasksGuardianDaemonHelperEnv  = "COMPOZY_TEST_ARTIFACT_GUARDIAN_DAEMON_HELPER"
	validateTasksGuardianHomeRootEnv      = "COMPOZY_TEST_ARTIFACT_GUARDIAN_HOME_ROOT"
	validateTasksGuardianPIDPathEnv       = "COMPOZY_TEST_ARTIFACT_GUARDIAN_PID_PATH"
	validateTasksGuardianBuildSentinel    = ".compozy-test-artifact-guardian"
	validateTasksGuardianHomeSentinel     = ".compozy-test-artifact-home"
	validateTasksGuardianRegisterHome     = "register_home"
	validateTasksGuardianGracefulShutdown = "graceful_shutdown"
)

var (
	validateTasksBinaryOnce sync.Once
	validateTasksBinaryPath string
	validateTasksBinaryErr  error
	validateTasksBuildDir   string
	validateTasksGuardianMu sync.Mutex
	validateTasksGuardian   *exec.Cmd
	validateTasksGuardianIn *os.File
	validateTasksGuardErr   string
)

func TestMain(m *testing.M) {
	code := m.Run()
	if err := cleanupValidateTasksArtifacts(); err != nil {
		if _, writeErr := fmt.Fprintf(os.Stderr, "cleanup validate-tasks artifacts: %v\n", err); writeErr != nil {
			code = 2
		} else {
			code = 1
		}
	}
	os.Exit(code)
}

type validateTasksGuardianMessage struct {
	Kind     string `json:"kind"`
	HomeRoot string `json:"home_root,omitempty"`
}

func cleanupValidateTasksArtifacts() error {
	validateTasksGuardianMu.Lock()
	guardian := validateTasksGuardian
	guardianIn := validateTasksGuardianIn
	errorPath := validateTasksGuardErr
	buildDir := validateTasksBuildDir
	validateTasksGuardian = nil
	validateTasksGuardianIn = nil
	validateTasksGuardErr = ""
	validateTasksGuardianMu.Unlock()

	if guardian == nil {
		if strings.TrimSpace(buildDir) == "" {
			return nil
		}
		if err := os.RemoveAll(buildDir); err != nil {
			return fmt.Errorf("remove unguarded build directory %q: %w", buildDir, err)
		}
		return nil
	}

	var cleanupErrs []error
	if err := writeValidateTasksGuardianMessage(guardianIn, validateTasksGuardianMessage{
		Kind: validateTasksGuardianGracefulShutdown,
	}); err != nil {
		cleanupErrs = append(cleanupErrs, err)
	}
	if err := guardianIn.Close(); err != nil {
		cleanupErrs = append(cleanupErrs, fmt.Errorf("close artifact guardian input: %w", err))
	}
	if err := waitForValidateTasksArtifactGuardian(guardian); err != nil {
		report, readErr := os.ReadFile(errorPath)
		if readErr == nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("artifact guardian: %s", strings.TrimSpace(string(report))))
		} else if !errors.Is(readErr, os.ErrNotExist) {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("read artifact guardian report: %w", readErr))
		}
		cleanupErrs = append(cleanupErrs, err)
	}
	if err := os.Remove(errorPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		cleanupErrs = append(cleanupErrs, fmt.Errorf("remove artifact guardian report: %w", err))
	}
	return errors.Join(cleanupErrs...)
}

func startValidateTasksArtifactGuardian(buildDir string) error {
	cleanBuildDir, err := validateTasksGuardianCleanupRoot(buildDir)
	if err != nil {
		return fmt.Errorf("validate guardian build directory: %w", err)
	}
	if err := os.MkdirAll(cleanBuildDir, 0o755); err != nil {
		return fmt.Errorf("create guardian build directory: %w", err)
	}
	if err := os.WriteFile(
		filepath.Join(cleanBuildDir, validateTasksGuardianBuildSentinel),
		[]byte("owned by internal/cli artifact guardian\n"),
		0o600,
	); err != nil {
		return fmt.Errorf("write guardian build sentinel: %w", err)
	}

	testExecutable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve test executable: %w", err)
	}
	guardianInput, parentInput, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("create artifact guardian pipe: %w", err)
	}
	errorPath := cleanBuildDir + ".guardian-error"
	if err := os.Remove(errorPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.Join(
			fmt.Errorf("remove stale artifact guardian report: %w", err),
			closeValidateTasksGuardianPipe("guardian input", guardianInput),
			closeValidateTasksGuardianPipe("parent input", parentInput),
		)
	}

	guardian := exec.CommandContext(
		context.Background(),
		testExecutable,
		"-test.run=^TestValidateTasksArtifactGuardian$",
	)
	guardian.Env = append(os.Environ(),
		validateTasksGuardianModeEnv+"=1",
		validateTasksGuardianBuildDirEnv+"="+cleanBuildDir,
		validateTasksGuardianErrorPathEnv+"="+errorPath,
	)
	guardian.Stdin = guardianInput
	guardian.SysProcAttr = daemonLaunchSysProcAttr()
	if err := guardian.Start(); err != nil {
		return errors.Join(
			fmt.Errorf("start artifact guardian: %w", err),
			closeValidateTasksGuardianPipe("guardian input", guardianInput),
			closeValidateTasksGuardianPipe("parent input", parentInput),
		)
	}
	if err := guardianInput.Close(); err != nil {
		killErr := guardian.Process.Kill()
		waitErr := guardian.Wait()
		return errors.Join(
			fmt.Errorf("close parent copy of artifact guardian input: %w", err),
			killErr,
			waitErr,
			closeValidateTasksGuardianPipe("parent input", parentInput),
		)
	}

	validateTasksGuardianMu.Lock()
	defer validateTasksGuardianMu.Unlock()
	if validateTasksGuardian != nil {
		killErr := guardian.Process.Kill()
		waitErr := guardian.Wait()
		return errors.Join(
			errors.New("artifact guardian already started"),
			killErr,
			waitErr,
			closeValidateTasksGuardianPipe("parent input", parentInput),
		)
	}
	validateTasksGuardian = guardian
	validateTasksGuardianIn = parentInput
	validateTasksGuardErr = errorPath
	return nil
}

func registerValidateTasksGuardianHomeRoot(homeRoot string) error {
	cleanHomeRoot, err := validateTasksGuardianCleanupRoot(homeRoot)
	if err != nil {
		return fmt.Errorf("validate guardian home root: %w", err)
	}
	if err := os.MkdirAll(cleanHomeRoot, 0o755); err != nil {
		return fmt.Errorf("create guardian home root: %w", err)
	}
	if err := os.WriteFile(
		filepath.Join(cleanHomeRoot, validateTasksGuardianHomeSentinel),
		[]byte("owned by internal/cli artifact guardian\n"),
		0o600,
	); err != nil {
		return fmt.Errorf("write guardian home sentinel: %w", err)
	}

	validateTasksGuardianMu.Lock()
	defer validateTasksGuardianMu.Unlock()
	if validateTasksGuardian == nil || validateTasksGuardianIn == nil {
		return errors.New("artifact guardian is not running")
	}
	return writeValidateTasksGuardianMessage(validateTasksGuardianIn, validateTasksGuardianMessage{
		Kind:     validateTasksGuardianRegisterHome,
		HomeRoot: cleanHomeRoot,
	})
}

func registerValidateTasksGuardianHome(t *testing.T, homeRoot string) {
	t.Helper()
	validateTasksBinary(t)
	if err := registerValidateTasksGuardianHomeRoot(homeRoot); err != nil {
		t.Fatalf("register validate-tasks guardian home: %v", err)
	}
}

func closeValidateTasksGuardianPipe(name string, pipe *os.File) error {
	if pipe == nil {
		return nil
	}
	if err := pipe.Close(); err != nil {
		return fmt.Errorf("close artifact guardian %s: %w", name, err)
	}
	return nil
}

func writeValidateTasksGuardianMessage(writer io.Writer, message validateTasksGuardianMessage) error {
	if writer == nil {
		return errors.New("artifact guardian input is unavailable")
	}
	if err := json.NewEncoder(writer).Encode(message); err != nil {
		return fmt.Errorf("write artifact guardian message %q: %w", message.Kind, err)
	}
	return nil
}

func waitForValidateTasksArtifactGuardian(guardian *exec.Cmd) error {
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- guardian.Wait()
	}()

	timer := time.NewTimer(defaultDaemonStartupTimeout + 5*time.Second)
	defer timer.Stop()
	select {
	case err := <-waitCh:
		if err != nil {
			return fmt.Errorf("wait for artifact guardian: %w", err)
		}
		return nil
	case <-timer.C:
		killErr := guardian.Process.Kill()
		waitErr := <-waitCh
		return errors.Join(
			errors.New("artifact guardian did not exit before cleanup deadline"),
			killErr,
			waitErr,
		)
	}
}

func runValidateTasksArtifactGuardian(input io.Reader, buildDir string) error {
	cleanBuildDir, err := validateTasksGuardianCleanupRoot(buildDir)
	if err != nil {
		return fmt.Errorf("validate guardian build directory: %w", err)
	}

	homeRoots := make(map[string]struct{})
	graceful := false
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		var message validateTasksGuardianMessage
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			return fmt.Errorf("decode guardian message: %w", err)
		}
		switch message.Kind {
		case validateTasksGuardianRegisterHome:
			cleanHomeRoot, err := validateTasksGuardianCleanupRoot(message.HomeRoot)
			if err != nil {
				return fmt.Errorf("validate registered guardian home: %w", err)
			}
			homeRoots[cleanHomeRoot] = struct{}{}
		case validateTasksGuardianGracefulShutdown:
			graceful = true
		default:
			return fmt.Errorf("unknown guardian message kind %q", message.Kind)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read guardian messages: %w", err)
	}

	if err := cleanupValidateTasksGuardianHomes(homeRoots, graceful); err != nil {
		return err
	}
	if err := removeValidateTasksGuardianRoot(cleanBuildDir, validateTasksGuardianBuildSentinel); err != nil {
		return fmt.Errorf("remove guardian build directory: %w", err)
	}
	return nil
}

func cleanupValidateTasksGuardianHomes(homeRoots map[string]struct{}, graceful bool) error {
	pending := make(map[string]struct{}, len(homeRoots))
	for homeRoot := range homeRoots {
		pending[homeRoot] = struct{}{}
	}
	if len(pending) == 0 {
		return nil
	}

	killedPIDs := make(map[int]struct{})
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(defaultDaemonStartupTimeout)
	defer timer.Stop()
	deadlineExpired := false
	for len(pending) > 0 {
		for homeRoot := range pending {
			done, err := cleanupValidateTasksGuardianHome(homeRoot, graceful || deadlineExpired, killedPIDs)
			if err != nil {
				return err
			}
			if done {
				delete(pending, homeRoot)
			}
		}
		if len(pending) == 0 {
			return nil
		}
		if deadlineExpired {
			return fmt.Errorf("guardian cleanup deadline expired for homes: %s", joinGuardianHomeRoots(pending))
		}

		select {
		case <-ticker.C:
		case <-timer.C:
			deadlineExpired = true
		}
	}
	return nil
}

func cleanupValidateTasksGuardianHome(
	homeRoot string,
	allowMissingInfoCleanup bool,
	killedPIDs map[int]struct{},
) (bool, error) {
	if _, err := os.Stat(homeRoot); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, fmt.Errorf("stat guardian home %q: %w", homeRoot, err)
	}
	if err := requireValidateTasksGuardianSentinel(homeRoot, validateTasksGuardianHomeSentinel); err != nil {
		return false, err
	}

	paths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(homeRoot, compozyconfig.DirName))
	if err != nil {
		return false, fmt.Errorf("resolve guardian daemon paths for %q: %w", homeRoot, err)
	}
	info, err := daemon.ReadInfo(paths.InfoPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if !allowMissingInfoCleanup {
				return false, nil
			}
			if err := removeValidateTasksGuardianRoot(homeRoot, validateTasksGuardianHomeSentinel); err != nil {
				return false, fmt.Errorf("remove guardian home %q: %w", homeRoot, err)
			}
			return true, nil
		}
		return false, fmt.Errorf("read guardian daemon info for %q: %w", homeRoot, err)
	}

	if daemon.ProcessAlive(info.PID) {
		if _, killed := killedPIDs[info.PID]; !killed {
			process, err := os.FindProcess(info.PID)
			if err != nil {
				return false, fmt.Errorf("find guardian daemon pid %d: %w", info.PID, err)
			}
			if err := process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				return false, fmt.Errorf("kill guardian daemon pid %d: %w", info.PID, err)
			}
			killedPIDs[info.PID] = struct{}{}
		}
		return false, nil
	}

	if err := removeValidateTasksGuardianRoot(homeRoot, validateTasksGuardianHomeSentinel); err != nil {
		return false, fmt.Errorf("remove guardian home %q: %w", homeRoot, err)
	}
	return true, nil
}

func validateTasksGuardianCleanupRoot(path string) (string, error) {
	clean := strings.TrimSpace(path)
	if clean == "" {
		return "", errors.New("cleanup root is required")
	}
	absPath, err := filepath.Abs(clean)
	if err != nil {
		return "", fmt.Errorf("resolve cleanup root %q: %w", clean, err)
	}
	volumeRoot := filepath.Clean(filepath.VolumeName(absPath) + string(os.PathSeparator))
	if filepath.Clean(absPath) == volumeRoot {
		return "", fmt.Errorf("refusing filesystem root cleanup %q", absPath)
	}
	return filepath.Clean(absPath), nil
}

func requireValidateTasksGuardianSentinel(root string, sentinel string) error {
	if _, err := os.Stat(filepath.Join(root, sentinel)); err != nil {
		return fmt.Errorf("guardian cleanup sentinel missing from %q: %w", root, err)
	}
	return nil
}

func removeValidateTasksGuardianRoot(root string, sentinel string) error {
	if _, err := os.Stat(root); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := requireValidateTasksGuardianSentinel(root, sentinel); err != nil {
		return err
	}
	return os.RemoveAll(root)
}

func joinGuardianHomeRoots(homeRoots map[string]struct{}) string {
	paths := make([]string, 0, len(homeRoots))
	for path := range homeRoots {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return strings.Join(paths, ", ")
}

func TestValidateTasksArtifactGuardianReapsDaemonAfterOwnerExit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	buildDir := filepath.Join(root, "build")
	homeRoot := filepath.Join(root, "home")
	pidPath := filepath.Join(root, "daemon.pid")
	errorPath := filepath.Join(root, "guardian.err")

	cmd := exec.CommandContext(
		context.Background(),
		os.Args[0],
		"-test.run=^TestValidateTasksGuardianOwnerHelper$",
	)
	cmd.Env = append(os.Environ(),
		validateTasksGuardianOwnerHelperEnv+"=1",
		validateTasksGuardianBuildDirEnv+"="+buildDir,
		validateTasksGuardianErrorPathEnv+"="+errorPath,
		validateTasksGuardianHomeRootEnv+"="+homeRoot,
		validateTasksGuardianPIDPathEnv+"="+pidPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run abrupt-owner helper: %v\n%s", err, output)
	}

	pidData, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("read daemon helper pid: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		t.Fatalf("parse daemon helper pid: %v", err)
	}

	waitForValidateTasksGuardianCleanup(t, pid, buildDir, homeRoot, errorPath)
}

func TestRunValidateTasksArtifactGuardianRejectsInvalidControlMessage(t *testing.T) {
	t.Parallel()

	buildDir := t.TempDir()
	err := runValidateTasksArtifactGuardian(strings.NewReader("{not-json}\n"), buildDir)
	if err == nil || !strings.Contains(err.Error(), "decode guardian message") {
		t.Fatalf("runValidateTasksArtifactGuardian() error = %v, want decode guardian message error", err)
	}
	if _, statErr := os.Stat(buildDir); statErr != nil {
		t.Fatalf("guardian removed build directory after invalid control message: %v", statErr)
	}
}

func TestValidateTasksArtifactGuardian(t *testing.T) {
	if os.Getenv(validateTasksGuardianModeEnv) != "1" {
		t.Skip("artifact guardian helper process")
	}

	buildDir := os.Getenv(validateTasksGuardianBuildDirEnv)
	errorPath := os.Getenv(validateTasksGuardianErrorPathEnv)
	if err := runValidateTasksArtifactGuardian(os.Stdin, buildDir); err != nil {
		if writeErr := os.WriteFile(errorPath, []byte(err.Error()+"\n"), 0o600); writeErr != nil {
			t.Fatalf("artifact guardian failed: %v; write error report: %v", err, writeErr)
		}
		t.Fatal(err)
	}
}

func TestValidateTasksGuardianOwnerHelper(t *testing.T) {
	if os.Getenv(validateTasksGuardianOwnerHelperEnv) != "1" {
		t.Skip("artifact guardian owner helper process")
	}

	buildDir := os.Getenv(validateTasksGuardianBuildDirEnv)
	homeRoot := os.Getenv(validateTasksGuardianHomeRootEnv)
	pidPath := os.Getenv(validateTasksGuardianPIDPathEnv)
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatalf("create guardian build directory: %v", err)
	}
	if err := startValidateTasksArtifactGuardian(buildDir); err != nil {
		t.Fatalf("start artifact guardian: %v", err)
	}
	if err := registerValidateTasksGuardianHomeRoot(homeRoot); err != nil {
		t.Fatalf("register guardian home: %v", err)
	}

	daemonCommand := exec.CommandContext(
		context.Background(),
		os.Args[0],
		"-test.run=^TestValidateTasksGuardianDaemonHelper$",
	)
	daemonCommand.Env = append(os.Environ(), validateTasksGuardianDaemonHelperEnv+"=1")
	daemonCommand.SysProcAttr = daemonLaunchSysProcAttr()
	if err := daemonCommand.Start(); err != nil {
		t.Fatalf("start daemon helper: %v", err)
	}
	daemonPID := daemonCommand.Process.Pid
	if err := daemonCommand.Process.Release(); err != nil {
		t.Fatalf("release daemon helper: %v", err)
	}

	paths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(homeRoot, compozyconfig.DirName))
	if err != nil {
		t.Fatalf("resolve guardian home paths: %v", err)
	}
	if err := compozyconfig.EnsureHomeLayout(paths); err != nil {
		t.Fatalf("create guardian home layout: %v", err)
	}
	if err := daemon.WriteInfo(paths.InfoPath, daemon.Info{
		PID:       daemonPID,
		StartedAt: time.Now().UTC(),
		State:     daemon.ReadyStateReady,
	}); err != nil {
		t.Fatalf("write daemon helper info: %v", err)
	}
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(daemonPID)+"\n"), 0o600); err != nil {
		t.Fatalf("write daemon helper pid: %v", err)
	}

	os.Exit(0)
}

func TestValidateTasksGuardianDaemonHelper(t *testing.T) {
	if os.Getenv(validateTasksGuardianDaemonHelperEnv) != "1" {
		t.Skip("artifact guardian daemon helper process")
	}

	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for daemon helper: %v", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			t.Errorf("close daemon helper listener: %v", err)
		}
	}()
	if _, err := listener.Accept(); err != nil {
		t.Fatalf("accept daemon helper connection: %v", err)
	}
}

func waitForValidateTasksGuardianCleanup(
	t *testing.T,
	pid int,
	buildDir string,
	homeRoot string,
	errorPath string,
) {
	t.Helper()

	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(defaultDaemonStartupTimeout + 5*time.Second)
	defer timer.Stop()
	for {
		buildMissing := pathIsMissing(buildDir)
		homeMissing := pathIsMissing(homeRoot)
		if !daemon.ProcessAlive(pid) && buildMissing && homeMissing {
			return
		}

		select {
		case <-ticker.C:
		case <-timer.C:
			report, _ := os.ReadFile(errorPath)
			t.Fatalf(
				"guardian cleanup timed out: pid_alive=%t build_missing=%t home_missing=%t report=%q",
				daemon.ProcessAlive(pid),
				buildMissing,
				homeMissing,
				strings.TrimSpace(string(report)),
			)
		}
	}
}

func pathIsMissing(path string) bool {
	_, err := os.Stat(path)
	return errors.Is(err, os.ErrNotExist)
}

func TestValidateTasksCommandJSONMixedFixture(t *testing.T) {
	workspaceRoot, tasksDir := makeValidateTasksWorkspace(t, "demo")
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{"status: pending", "title: Valid One", "type: backend", "complexity: low"},
		"# Task 1: Valid One",
	))
	writeRawTaskFileForCLI(t, tasksDir, "task_02.md", cliTaskMarkdown(
		[]string{"status: pending", "title: Valid Two", "type: docs", "complexity: medium"},
		"# Task 2: Valid Two",
	))
	invalidTitlePath := filepath.Join(tasksDir, "task_03.md")
	writeRawTaskFileForCLI(t, tasksDir, "task_03.md", cliTaskMarkdown(
		[]string{"status: pending", "type: backend", "complexity: low"},
		"# Task 3: Missing Title",
	))
	invalidTypePath := filepath.Join(tasksDir, "task_04.md")
	writeRawTaskFileForCLI(t, tasksDir, "task_04.md", cliTaskMarkdown(
		[]string{"status: pending", "title: Invalid Type", "type: nope", "complexity: low"},
		"# Task 4: Invalid Type",
	))

	stdout, stderr, exitCode := runValidateTasksCommand(
		t,
		workspaceRoot,
		"tasks",
		"validate",
		"--tasks-dir",
		tasksDir,
		"--format",
		"json",
	)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}

	var payload validateTasksOutput
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal json output: %v\nstdout:\n%s", err, stdout)
	}
	if payload.FixPrompt == "" {
		t.Fatal("expected non-empty fix_prompt")
	}

	gotPaths := distinctPaths(payload.Issues)
	wantPaths := []string{invalidTitlePath, invalidTypePath}
	slices.Sort(gotPaths)
	slices.Sort(wantPaths)
	if !slices.Equal(gotPaths, wantPaths) {
		t.Fatalf("unexpected invalid paths\nwant: %#v\ngot:  %#v", wantPaths, gotPaths)
	}
}

func TestValidateTasksCommandAllValid(t *testing.T) {
	workspaceRoot, tasksDir := makeValidateTasksWorkspace(t, "demo")
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{"status: pending", "title: Valid One", "type: backend", "complexity: low"},
		"# Task 1: Valid One",
	))
	writeRawTaskFileForCLI(t, tasksDir, "task_02.md", cliTaskMarkdown(
		[]string{"status: blocked", "title: Valid Two", "type: docs", "complexity: medium"},
		"# Task 2: Valid Two",
	))

	stdout, stderr, exitCode := runValidateTasksCommand(t, workspaceRoot, "tasks", "validate", "--tasks-dir", tasksDir)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	if !strings.Contains(stdout, "all tasks valid") {
		t.Fatalf("expected success output, got %q", stdout)
	}
}

func TestValidateTasksCommandMissingDir(t *testing.T) {
	workspaceRoot, _ := makeValidateTasksWorkspace(t, "demo")
	missingDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "missing")

	stdout, stderr, exitCode := runValidateTasksCommand(
		t,
		workspaceRoot,
		"tasks",
		"validate",
		"--tasks-dir",
		missingDir,
	)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout for missing-dir failure, got %q", stdout)
	}
	if !strings.Contains(stderr, "read tasks directory") || !strings.Contains(stderr, missingDir) {
		t.Fatalf("expected clear missing-dir error, got %q", stderr)
	}
}

func TestWriteValidateTasksJSONAndHelpers(t *testing.T) {
	t.Parallel()

	registry, err := tasks.NewRegistry(nil)
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	report := tasks.Report{
		TasksDir: "/tmp/tasks",
		Scanned:  1,
		Issues: []tasks.Issue{
			{
				Path:    "/tmp/tasks/task_01.md",
				Field:   "title",
				Message: "title is required",
			},
		},
	}

	var out bytes.Buffer
	if err := writeValidateTasksJSON(&out, report, registry); err != nil {
		t.Fatalf("write validate tasks json: %v", err)
	}

	var payload validateTasksOutput
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode validate tasks json: %v", err)
	}
	if payload.OK {
		t.Fatal("expected invalid payload")
	}
	if payload.FixPrompt == "" {
		t.Fatal("expected fix prompt in json payload")
	}
	if got := validateTasksMessage(tasks.Report{Scanned: 1}); got != "all tasks valid" {
		t.Fatalf("unexpected ok message: %q", got)
	}
	if got := validateTasksMessage(tasks.Report{}); got != "no tasks found" {
		t.Fatalf("unexpected no-tasks message: %q", got)
	}
}

func TestValidateTasksCommandTrimsFormatBeforeSelectingJSONWriter(t *testing.T) {
	workspaceRoot, tasksDir := makeValidateTasksWorkspace(t, "demo")
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{"status: pending", "type: backend", "complexity: low"},
		"# Task 1: Missing Title",
	))

	stdout, stderr, exitCode := runValidateTasksCommand(
		t,
		workspaceRoot,
		"tasks",
		"validate",
		"--tasks-dir",
		tasksDir,
		"--format",
		" json ",
	)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}

	var payload validateTasksOutput
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf(
			"expected trimmed json format to produce json output: %v\nstdout:\n%s\nstderr:\n%s",
			err,
			stdout,
			stderr,
		)
	}
}

func TestValidateTasksCommandResolvesRelativeTasksDirFromWorkspaceRoot(t *testing.T) {
	workspaceRoot, tasksDir := makeValidateTasksWorkspace(t, "demo")
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{"status: pending", "title: Valid One", "type: backend", "complexity: low"},
		"# Task 1: Valid One",
	))

	nested := filepath.Join(workspaceRoot, "pkg", "feature")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested dir: %v", err)
	}

	stdout, stderr, exitCode := runValidateTasksCommand(
		t,
		nested,
		"tasks",
		"validate",
		"--tasks-dir",
		filepath.Join(".compozy", "tasks", "demo"),
	)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}
	if !strings.Contains(stdout, "all tasks valid") {
		t.Fatalf("expected success output, got %q", stdout)
	}
}

func TestResolveTaskWorkflowDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	got, err := resolveTaskWorkflowDir(root, "demo", "")
	if err != nil {
		t.Fatalf("resolve task workflow dir from name: %v", err)
	}
	want := filepath.Join(root, ".compozy", "tasks", "demo")
	if got != want {
		t.Fatalf("unexpected resolved dir\nwant: %q\ngot:  %q", want, got)
	}

	if _, err := resolveTaskWorkflowDir(root, "", ""); err == nil {
		t.Fatal("expected missing-input error")
	}
}

func runValidateTasksCommand(t *testing.T, dir string, args ...string) (string, string, int) {
	t.Helper()
	return runCLICommand(t, dir, args...)
}

func runCLICommand(t *testing.T, dir string, args ...string) (string, string, int) {
	t.Helper()

	cmd := exec.CommandContext(context.Background(), validateTasksBinary(t), args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("run validate-tasks command: %v", err)
	}
	return stdout.String(), stderr.String(), exitErr.ExitCode()
}

func validateTasksBinary(t *testing.T) string {
	t.Helper()

	validateTasksBinaryOnce.Do(func() {
		repoRoot, err := validateTasksRepoRoot()
		if err != nil {
			validateTasksBinaryErr = err
			return
		}

		buildDir, err := os.MkdirTemp("", "compozy-validate-tasks-*")
		if err != nil {
			validateTasksBinaryErr = err
			return
		}
		validateTasksBuildDir = buildDir

		validateTasksBinaryPath = filepath.Join(buildDir, "compozy")
		cmd := exec.CommandContext(context.Background(), "go", "build", "-o", validateTasksBinaryPath, "./cmd/compozy")
		cmd.Dir = repoRoot
		cmd.Env = buildCLITestCommandEnv()
		output, err := cmd.CombinedOutput()
		if err != nil {
			buildErr := fmt.Errorf("build compozy binary: %w\n%s", err, output)
			removeErr := os.RemoveAll(buildDir)
			validateTasksBuildDir = ""
			validateTasksBinaryErr = errors.Join(buildErr, removeErr)
			return
		}
		if err := startValidateTasksArtifactGuardian(buildDir); err != nil {
			removeErr := os.RemoveAll(buildDir)
			validateTasksBuildDir = ""
			validateTasksBinaryErr = errors.Join(err, removeErr)
		}
	})

	if validateTasksBinaryErr != nil {
		t.Fatal(validateTasksBinaryErr)
	}
	return validateTasksBinaryPath
}

func buildCLITestCommandEnv() []string {
	env := os.Environ()
	if strings.TrimSpace(originalCLIHome) == "" {
		return env
	}

	prefix := "HOME="
	filtered := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			continue
		}
		filtered = append(filtered, entry)
	}
	filtered = append(filtered, prefix+originalCLIHome)
	return filtered
}

func validateTasksRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.Join(cwd, "..", "..")), nil
}

func makeValidateTasksWorkspace(t *testing.T, name string) (string, string) {
	t.Helper()

	root := t.TempDir()
	tasksDir := filepath.Join(root, ".compozy", "tasks", name)
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}
	return root, tasksDir
}

func writeTaskWorkflowForCLI(t *testing.T, workspaceRoot string, slug string) string {
	t.Helper()

	tasksDir := filepath.Join(workspaceRoot, ".compozy", "tasks", slug)
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir task workflow %s: %v", slug, err)
	}
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{
			"status: pending",
			"title: " + slug + " Task",
			"type: backend",
			"complexity: low",
		},
		"# Task 1: "+slug+" Task",
	))
	return tasksDir
}

func cliTaskMarkdown(frontMatter []string, h1 string) string {
	lines := []string{"---"}
	lines = append(lines, frontMatter...)
	lines = append(lines, "---", "", h1, "", "Body.")
	return strings.Join(lines, "\n") + "\n"
}

func writeRawTaskFileForCLI(t *testing.T, tasksDir, name, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(tasksDir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func distinctPaths(issues []tasks.Issue) []string {
	seen := make(map[string]struct{}, len(issues))
	paths := make([]string, 0, len(issues))
	for _, issue := range issues {
		if _, ok := seen[issue.Path]; ok {
			continue
		}
		seen[issue.Path] = struct{}{}
		paths = append(paths, issue.Path)
	}
	return paths
}
