package cli

import (
	"bufio"
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
