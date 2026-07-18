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
	validateTasksGuardianDaemonPortEnv    = "COMPOZY_TEST_ARTIFACT_GUARDIAN_DAEMON_PORT_PATH"
	validateTasksGuardianHomeRootEnv      = "COMPOZY_TEST_ARTIFACT_GUARDIAN_HOME_ROOT"
	validateTasksGuardianPIDPathEnv       = "COMPOZY_TEST_ARTIFACT_GUARDIAN_PID_PATH"
	validateTasksGuardianBuildSentinel    = ".compozy-test-artifact-guardian"
	validateTasksGuardianHomeSentinel     = ".compozy-test-artifact-home"
	validateTasksGuardianRegisterHome     = "register_home"
	validateTasksGuardianGracefulShutdown = "graceful_shutdown"
)

// guardianOwnershipProbeTimeout bounds each ownership probe (see
// daemonEndpointReachable). Localhost dials resolve in microseconds, so this is
// only a backstop for a half-open endpoint.
const guardianOwnershipProbeTimeout = 500 * time.Millisecond

// guardianDrainGrace is the bounded window after the reap deadline during which
// the loop keeps polling so a daemon first killed on the deadline pass is
// observed exiting before its home is reported as still pending.
const guardianDrainGrace = 2 * time.Second

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

	// Reap homes and remove the build directory independently: a single damaged
	// home must not strand the build directory (or the other homes) on disk.
	homeErr := cleanupValidateTasksGuardianHomes(homeRoots, graceful)
	var buildErr error
	if err := removeValidateTasksGuardianRoot(cleanBuildDir, validateTasksGuardianBuildSentinel); err != nil {
		buildErr = fmt.Errorf("remove guardian build directory: %w", err)
	}
	return errors.Join(homeErr, buildErr)
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
	softDeadline := time.NewTimer(defaultDaemonStartupTimeout)
	defer softDeadline.Stop()
	hardDeadline := time.NewTimer(defaultDaemonStartupTimeout + guardianDrainGrace)
	defer hardDeadline.Stop()
	softExpired := false
	var errs []error
	for len(pending) > 0 {
		for homeRoot := range pending {
			done, err := cleanupValidateTasksGuardianHome(homeRoot, graceful || softExpired, killedPIDs)
			if err != nil {
				// Record and drop this home so one damaged artifact cannot block
				// reaping the rest; the joined error still surfaces to TestMain.
				errs = append(errs, err)
				delete(pending, homeRoot)
				continue
			}
			if done {
				delete(pending, homeRoot)
			}
		}
		if len(pending) == 0 {
			break
		}

		// After the soft deadline, allowMissingInfoCleanup is enabled so unverifiable
		// homes resolve; the loop keeps polling through the drain grace so a daemon
		// killed on that pass is observed exiting before the hard deadline expires.
		select {
		case <-ticker.C:
		case <-softDeadline.C:
			softExpired = true
		case <-hardDeadline.C:
			errs = append(errs, fmt.Errorf(
				"guardian cleanup deadline expired for homes: %s",
				joinGuardianHomeRoots(pending),
			))
			return errors.Join(errs...)
		}
	}
	return errors.Join(errs...)
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
			return removeValidateTasksGuardianHomeDone(homeRoot)
		}
		return false, fmt.Errorf("read guardian daemon info for %q: %w", homeRoot, err)
	}

	if !daemon.ProcessAlive(info.PID) {
		// The recorded daemon has exited; its home is safe to remove.
		return removeValidateTasksGuardianHomeDone(homeRoot)
	}
	if _, killed := killedPIDs[info.PID]; killed {
		// We already signaled this instance. Re-verify ownership instead of
		// trusting the PID: if the endpoint still answers, our daemon is still
		// exiting, so keep polling; if it no longer answers, our daemon is gone
		// (a recycled PID would not be holding our endpoint) and the home is safe
		// to remove. This keeps the drain immune to PID reuse.
		if daemonEndpointReachable(info) {
			return false, nil
		}
		return removeValidateTasksGuardianHomeDone(homeRoot)
	}
	killed, err := killGuardianDaemonInstance(info)
	if err != nil {
		return false, err
	}
	if killed {
		killedPIDs[info.PID] = struct{}{}
		return false, nil
	}
	if !daemon.ProcessAlive(info.PID) {
		// The daemon exited while we confirmed ownership; nothing left to kill.
		return removeValidateTasksGuardianHomeDone(homeRoot)
	}
	// The pid is alive but not confirmed as our daemon — a recycled pid or a hung
	// endpoint. Never SIGKILL a pid we cannot confirm is ours. Keep polling; if it
	// stays unverifiable past the deadline, surface an error and preserve the home
	// rather than deleting metadata a later reap would need.
	if !allowMissingInfoCleanup {
		return false, nil
	}
	return false, fmt.Errorf(
		"guardian daemon pid %d is alive but its endpoint cannot be verified",
		info.PID,
	)
}

// killGuardianDaemonByPID confirms ownership via the daemon's endpoint, then
// signals info.PID directly. It is the portable fallback used where no pidfd
// (or equivalent instance handle) is available, so ownership is verified as
// tightly before the signal as the platform allows. Returns killed=false when
// the endpoint cannot be reached, i.e. the process is not confirmed as ours.
func killGuardianDaemonByPID(info daemon.Info) (bool, error) {
	if !daemonEndpointReachable(info) {
		return false, nil
	}
	process, err := os.FindProcess(info.PID)
	if err != nil {
		return false, fmt.Errorf("find guardian daemon pid %d: %w", info.PID, err)
	}
	if err := process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return false, fmt.Errorf("kill guardian daemon pid %d: %w", info.PID, err)
	}
	return true, nil
}

// removeValidateTasksGuardianHomeDone removes a guarded home and reports it as
// finished so the reap loop can stop tracking it.
func removeValidateTasksGuardianHomeDone(homeRoot string) (bool, error) {
	if err := removeValidateTasksGuardianRoot(homeRoot, validateTasksGuardianHomeSentinel); err != nil {
		return false, fmt.Errorf("remove guardian home %q: %w", homeRoot, err)
	}
	return true, nil
}

// daemonEndpointReachable reports whether the daemon recorded in info still
// answers on its own endpoint. It is the ownership gate for the guardian's
// SIGKILL: a recycled pid will not be holding the recorded socket or port, so
// an unreachable endpoint means the process at info.PID is not our daemon.
// Mirrors daemon.ProbeReady's socket-based liveness check.
func daemonEndpointReachable(info daemon.Info) bool {
	if socketPath := strings.TrimSpace(info.SocketPath); socketPath != "" && dialProbe("unix", socketPath) {
		return true
	}
	if info.HTTPPort > 0 && dialProbe("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(info.HTTPPort))) {
		return true
	}
	return false
}

// dialProbe reports whether address accepts a connection within the ownership
// probe timeout. Each probe gets its own deadline so one slow endpoint cannot
// starve the next.
func dialProbe(network, address string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), guardianOwnershipProbeTimeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
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
		return fmt.Errorf("stat guardian cleanup root %q: %w", root, err)
	}
	if err := requireValidateTasksGuardianSentinel(root, sentinel); err != nil {
		return err
	}
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("remove guardian cleanup root %q: %w", root, err)
	}
	return nil
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
	t.Run("Should reap the leaked daemon and artifacts after the owner exits abruptly", func(t *testing.T) {
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
	})
}

func TestRunValidateTasksArtifactGuardianRejectsInvalidControlMessage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "Should reject a malformed control message",
			input:   "{not-json}\n",
			wantErr: "decode guardian message",
		},
		{
			name:    "Should reject an unknown control message kind",
			input:   `{"kind":"bogus"}` + "\n",
			wantErr: "unknown guardian message kind",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			buildDir := t.TempDir()
			err := runValidateTasksArtifactGuardian(strings.NewReader(tc.input), buildDir)
			assertErrorContains(t, err, tc.wantErr)
			if _, statErr := os.Stat(buildDir); statErr != nil {
				t.Fatalf("guardian removed build directory after invalid control message: %v", statErr)
			}
		})
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

	daemonPortPath := pidPath + ".port"
	daemonCommand := exec.CommandContext(
		context.Background(),
		os.Args[0],
		"-test.run=^TestValidateTasksGuardianDaemonHelper$",
	)
	daemonCommand.Env = append(os.Environ(),
		validateTasksGuardianDaemonHelperEnv+"=1",
		validateTasksGuardianDaemonPortEnv+"="+daemonPortPath,
	)
	daemonCommand.SysProcAttr = daemonLaunchSysProcAttr()
	if err := daemonCommand.Start(); err != nil {
		t.Fatalf("start daemon helper: %v", err)
	}
	daemonPID := daemonCommand.Process.Pid
	if err := daemonCommand.Process.Release(); err != nil {
		t.Fatalf("release daemon helper: %v", err)
	}

	// The guardian's ownership probe dials this port, so record it in daemon.json
	// before advertising the daemon as ready.
	daemonPort, err := waitForDaemonHelperPort(daemonPortPath)
	if err != nil {
		t.Fatalf("resolve daemon helper port: %v", err)
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
		HTTPPort:  daemonPort,
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

	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("resolve daemon helper address: %v", err)
	}
	if err := writeDaemonHelperPort(os.Getenv(validateTasksGuardianDaemonPortEnv), port); err != nil {
		t.Fatalf("report daemon helper port: %v", err)
	}

	// Stay alive serving the guardian's ownership probes until it SIGKILLs us,
	// exactly as a leaked real daemon keeps its socket open until reaped.
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		if err := conn.Close(); err != nil {
			t.Errorf("close daemon helper probe connection: %v", err)
		}
	}
}

// writeDaemonHelperPort publishes the daemon helper's listening port atomically
// (temp file + rename) so waitForDaemonHelperPort never observes a partial write.
func writeDaemonHelperPort(path string, port string) error {
	clean := strings.TrimSpace(path)
	if clean == "" {
		return errors.New("daemon helper port path is required")
	}
	tmp := clean + ".tmp"
	if err := os.WriteFile(tmp, []byte(port+"\n"), 0o600); err != nil {
		return fmt.Errorf("write daemon helper port file: %w", err)
	}
	if err := os.Rename(tmp, clean); err != nil {
		removeErr := os.Remove(tmp)
		if removeErr != nil {
			removeErr = fmt.Errorf("remove temporary daemon helper port file %q: %w", tmp, removeErr)
		}
		return errors.Join(fmt.Errorf("rename daemon helper port file: %w", err), removeErr)
	}
	return nil
}

// waitForDaemonHelperPort polls for the port file the daemon helper writes,
// returning once it is readable or the startup deadline passes.
func waitForDaemonHelperPort(path string) (int, error) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(defaultDaemonStartupTimeout)
	defer timer.Stop()
	for {
		data, err := os.ReadFile(path)
		switch {
		case err == nil:
			text := strings.TrimSpace(string(data))
			port, convErr := strconv.Atoi(text)
			if convErr != nil {
				return 0, fmt.Errorf("parse daemon helper port %q: %w", text, convErr)
			}
			if port <= 0 {
				return 0, fmt.Errorf("daemon helper port must be positive: %d", port)
			}
			return port, nil
		case errors.Is(err, os.ErrNotExist):
			// The helper has not reported its port yet; keep polling below.
		default:
			return 0, fmt.Errorf("read daemon helper port: %w", err)
		}

		select {
		case <-ticker.C:
		case <-timer.C:
			return 0, errors.New("daemon helper did not report its port before deadline")
		}
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

// reapValidateTasksGuardianHome runs the guardian's daemon-aware cleanup for a
// single registered home synchronously: it stops any daemon still running there
// before removing the home, so a per-test cleanup never deletes the daemon.json
// the package guardian would otherwise need to reap a still-live daemon.
func reapValidateTasksGuardianHome(homeRoot string) error {
	return cleanupValidateTasksGuardianHomes(map[string]struct{}{homeRoot: {}}, true)
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want substring %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want substring %q", err, want)
	}
}
