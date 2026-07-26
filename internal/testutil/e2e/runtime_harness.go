package e2e

import (
	"context"

	"errors"
	"fmt"
	"io"

	"net"
	"net/http"

	"os"
	"os/exec"
	"path/filepath"

	"strings"
	"sync"

	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/procutil"

	"github.com/compozy/agh/internal/testutil/acpmock"

	"golang.org/x/sys/execabs"
)

const (
	runtimeHarnessDoneKey       = "done"
	runtimeHarnessEventKey      = "event"
	runtimeHarnessHTTPUnixPath  = "http://unix"
	runtimeHarnessMessageKey    = "message"
	runtimeHarnessPermissionKey = "permission"
)

const (
	defaultStartTimeout = 20 * time.Second
	defaultPollInterval = 100 * time.Millisecond
	maxStartAttempts    = 3
	windowsGOOS         = "windows"
	daemonBinaryEnvVar  = "AGH_TEST_DAEMON_BIN"
	runtimeManifestName = "runtime-manifest.json"
)

var (
	errDaemonExitedBeforeReadiness = errors.New("daemon exited before readiness")
	errSSEPredicateRequired        = errors.New("SSE predicate is required")
)

var (
	buildBinaryMu   sync.Mutex
	builtBinaryPath string
)

// RuntimeHarnessOptions configures one isolated daemon runtime.
type RuntimeHarnessOptions struct {
	BinaryPath       string
	HomePaths        aghconfig.HomePaths
	ConfigSeed       ConfigSeedOptions
	MockAgents       []MockAgentSpec
	Workspace        WorkspaceSeedOptions
	Env              map[string]string
	EnableNetwork    bool
	StartTimeout     time.Duration
	PollInterval     time.Duration
	ResolveWorkspace bool
}

type runtimeLayout struct {
	HomePaths     aghconfig.HomePaths
	Config        aghconfig.Config
	WorkspaceRoot string
	Artifacts     *ArtifactCollector
	MockAgents    map[string]acpmock.Registration
	Env           []string
}

// RuntimeHarness exposes the started daemon and its public product surfaces.
type RuntimeHarness struct {
	HomePaths     aghconfig.HomePaths
	Config        aghconfig.Config
	BinaryPath    string
	Artifacts     *ArtifactCollector
	WorkspaceRoot string
	WorkspaceID   string
	MockAgents    map[string]acpmock.Registration

	HTTPBaseURL string
	HTTPClient  *http.Client

	UDSBaseURL string
	UDSClient  *http.Client

	CLI *CLIClient

	process *exec.Cmd
	waitCh  <-chan error

	processLogPath string

	stopOnce sync.Once
	stopErr  error

	processWaitMu sync.Mutex
	processExited bool
	processErr    error
}

// CLIClient shells out to the real `agh` binary against the isolated runtime.
type CLIClient struct {
	binaryPath string
	env        []string
	workdir    string
}

// SSEEvent captures one parsed server-sent event record.
type SSEEvent struct {
	ID    string
	Event string
	Data  []byte
}

// StartRuntimeHarness boots an isolated daemon through the real CLI startup path.
func StartRuntimeHarness(t testing.TB, opts *RuntimeHarnessOptions) *RuntimeHarness {
	t.Helper()

	layout := prepareRuntimeLayout(t, opts)
	binaryPath := strings.TrimSpace(opts.BinaryPath)
	if binaryPath == "" {
		binaryPath = buildAGHBinary(t)
	}
	env, err := withRuntimeCLIEnv(layout.HomePaths, layout.Env, binaryPath)
	if err != nil {
		t.Fatalf("prepare runtime CLI env error = %v", err)
	}
	layout.Env = env
	harness := newRuntimeHarness(t, &layout, binaryPath)
	startRuntimeProcess(t, harness, layout.Env, opts)

	workspace, err := harness.ResolveWorkspace(context.Background(), layout.WorkspaceRoot)
	if err != nil {
		if stopErr := harness.Stop(context.Background()); stopErr != nil {
			t.Fatalf("stop runtime harness after workspace resolve failure error = %v", stopErr)
		}
		t.Fatalf("resolve workspace %q error = %v", layout.WorkspaceRoot, err)
	}
	harness.WorkspaceID = workspace.ID
	if _, err := harness.WriteRuntimeManifest(); err != nil {
		if stopErr := harness.Stop(context.Background()); stopErr != nil {
			t.Fatalf("stop runtime harness after runtime manifest failure error = %v", stopErr)
		}
		t.Fatalf("write runtime manifest error = %v", err)
	}

	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := harness.Stop(stopCtx); err != nil {
			t.Fatalf("stop runtime harness error = %v", err)
		}
	})

	return harness
}

func startRuntimeProcess(
	t testing.TB,
	harness *RuntimeHarness,
	env []string,
	opts *RuntimeHarnessOptions,
) {
	t.Helper()

	startTimeout := defaultDuration(opts.StartTimeout, defaultStartTimeout)
	pollInterval := defaultDuration(opts.PollInterval, defaultPollInterval)

	for attempt := 1; attempt <= maxStartAttempts; attempt++ {
		startDaemonProcess(t, harness, env)

		readyCtx, cancel := context.WithTimeout(context.Background(), startTimeout)
		err := harness.waitForReady(readyCtx, pollInterval)
		cancel()
		if err == nil {
			return
		}
		retryHTTPPort, retrySocketPath := harness.readinessFailureRetryReasons(err)

		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		cleanupErr := harness.cleanupFailedStart(cleanupCtx)
		cleanupCancel()
		if attempt == maxStartAttempts || (!retryHTTPPort && !retrySocketPath) {
			if cleanupErr != nil {
				t.Fatalf("cleanup failed start after readiness error = %v (readiness error = %v)", cleanupErr, err)
			}
			t.Fatalf("wait for daemon readiness error = %v", err)
		}
		if cleanupErr != nil {
			t.Fatalf("cleanup failed start before retry error = %v (readiness error = %v)", cleanupErr, err)
		}
		if retryHTTPPort {
			if err := harness.reseedRuntimeHTTPPort(t); err != nil {
				t.Fatalf("reseed runtime HTTP port error = %v", err)
			}
		}
		if retrySocketPath {
			if err := harness.reseedRuntimeSocketPath(t); err != nil {
				t.Fatalf("reseed runtime UDS socket error = %v", err)
			}
		}
	}
}

func newRuntimeHarness(t testing.TB, layout *runtimeLayout, binaryPath string) *RuntimeHarness {
	t.Helper()

	repoRoot := mustRepoRoot(t)
	processLogPath := filepath.Join(layout.Artifacts.RootDir(), "daemon-process.log")
	return &RuntimeHarness{
		HomePaths:     layout.HomePaths,
		Config:        layout.Config,
		BinaryPath:    binaryPath,
		Artifacts:     layout.Artifacts,
		WorkspaceRoot: layout.WorkspaceRoot,
		MockAgents:    cloneMockAgentRegistrations(layout.MockAgents),
		HTTPBaseURL:   fmt.Sprintf("http://%s:%d", layout.Config.HTTP.Host, layout.Config.HTTP.Port),
		HTTPClient:    newHTTPClient(),
		UDSBaseURL:    runtimeHarnessHTTPUnixPath,
		UDSClient:     newUDSClient(layout.Config.Daemon.Socket),
		CLI: &CLIClient{
			binaryPath: binaryPath,
			env:        layout.Env,
			workdir:    repoRoot,
		},
		processLogPath: processLogPath,
	}
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: nil,
		},
	}
}

func newUDSClient(socketPath string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: nil,
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var dialer net.Dialer
				return dialer.DialContext(ctx, "unix", socketPath)
			},
		},
	}
}

func startDaemonProcess(t testing.TB, harness *RuntimeHarness, env []string) {
	t.Helper()

	processLogPath := harness.processLogPath
	if strings.TrimSpace(processLogPath) == "" {
		processLogPath = filepath.Join(harness.Artifacts.RootDir(), "daemon-process.log")
	}
	processLog, err := os.Create(processLogPath)
	if err != nil {
		t.Fatalf("os.Create(%q) error = %v", processLogPath, err)
	}

	// #nosec G204 -- test harness intentionally executes the built agh binary against isolated test state.
	cmd := execabs.CommandContext(
		context.Background(),
		harness.BinaryPath,
		"daemon", "start", "--foreground", "--exit-when-orphaned",
	)
	cmd.Env = append([]string(nil), env...)
	cmd.Stdout = processLog
	cmd.Stderr = processLog
	cmd.Dir = mustRepoRoot(t)
	// Own process group + exit-when-orphaned so an abnormally-terminated test
	// (SIGKILL / timeout) never leaks the daemon subtree.
	procutil.ConfigureCommandProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		if closeErr := processLog.Close(); closeErr != nil {
			t.Fatalf("start daemon process error = %v; close process log error = %v", err, closeErr)
		}
		t.Fatalf("start daemon process error = %v", err)
	}
	if err := procutil.RegisterCommandProcessGroup(cmd); err != nil {
		if cleanupErr := cleanupStartedDaemonProcess(cmd, processLog); cleanupErr != nil {
			t.Fatalf("register daemon process group error = %v; cleanup error = %v", err, cleanupErr)
		}
		t.Fatalf("register daemon process group error = %v", err)
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- errors.Join(cmd.Wait(), processLog.Close())
		close(waitCh)
	}()

	harness.process = cmd
	harness.waitCh = waitCh
}

func cleanupStartedDaemonProcess(cmd *exec.Cmd, processLog io.Closer) error {
	killErr := procutil.KillCommandProcessGroupAndWait(cmd, 5*time.Second)
	waitErr := cmd.Wait()
	exitErr, ok := errors.AsType[*exec.ExitError](waitErr)
	if ok && exitErr != nil {
		waitErr = nil
	}
	return errors.Join(killErr, waitErr, processLog.Close())
}

func prepareRuntimeLayout(t testing.TB, opts *RuntimeHarnessOptions) runtimeLayout {
	t.Helper()

	homePaths := opts.HomePaths
	if strings.TrimSpace(homePaths.HomeDir) == "" {
		homePaths = NewHomePaths(t)
	}
	configSeed := opts.ConfigSeed
	originalMutate := configSeed.Mutate
	configSeed.Mutate = func(cfg *aghconfig.Config) {
		if originalMutate != nil {
			originalMutate(cfg)
		}
		if len(opts.MockAgents) > 0 {
			ensureMockAgentProviderConfig(t, cfg)
		}
		if opts.EnableNetwork {
			cfg.Network.Enabled = true
		}
	}
	config := SeedConfig(t, homePaths, configSeed)
	workspaceRoot := SeedWorkspace(t, opts.Workspace)
	artifacts := NewArtifactCollector(t)
	mockAgents := registerMockAgents(t, homePaths, artifacts, opts.MockAgents)

	return runtimeLayout{
		HomePaths:     homePaths,
		Config:        config,
		WorkspaceRoot: workspaceRoot,
		Artifacts:     artifacts,
		MockAgents:    mockAgents,
		Env:           runtimeEnv(homePaths, opts.Env),
	}
}
