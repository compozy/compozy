package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	aghcontract "github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/procutil"
	"github.com/compozy/agh/internal/testutil/acpmock"
)

func TestRuntimeHarnessWaitForReadyUsesPublicSurfaces(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := writeJSONResponse(w, aghcontract.StatusPayload{
			Daemon: aghcontract.DaemonStatusPayload{
				Status:   "running",
				Socket:   "/tmp/agh.sock",
				HTTPHost: "127.0.0.1",
				HTTPPort: 2123,
			},
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	harness := &RuntimeHarness{
		HTTPBaseURL: server.URL,
		HTTPClient:  server.Client(),
		UDSBaseURL:  server.URL,
		UDSClient:   server.Client(),
		CLI: &CLIClient{
			binaryPath: writeCLIScript(t, `#!/bin/sh
printf '%s\n' '{"status":"running","socket":"/tmp/agh.sock","http_host":"127.0.0.1","http_port":2123}'
`),
			workdir: t.TempDir(),
		},
		waitCh: make(chan error),
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultStartTimeout)
	defer cancel()
	if err := harness.waitForReady(ctx, time.Millisecond); err != nil {
		t.Fatalf("waitForReady() error = %v", err)
	}
}

func TestRuntimeHarnessWaitForReadyReturnsExitError(t *testing.T) {
	t.Parallel()

	waitCh := make(chan error, 1)
	waitCh <- errors.New("daemon exited")

	harness := &RuntimeHarness{waitCh: waitCh}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := harness.waitForReady(ctx, time.Millisecond); err == nil {
		t.Fatal("waitForReady() error = nil, want non-nil")
	} else if !errors.Is(err, errDaemonExitedBeforeReadiness) {
		t.Fatalf("waitForReady() error = %v, want errDaemonExitedBeforeReadiness", err)
	}
}

func TestReadSSERecordsInferSemanticEventsFromJSONFrames(t *testing.T) {
	t.Parallel()

	stream := strings.NewReader(
		"data: {\"type\":\"text-delta\",\"delta\":\"partial\"}\n\n" +
			"data: {\"type\":\"data-agh-permission\",\"data\":{\"request_id\":\"req-1\"}}\n\n" +
			"data: {\"type\":\"data-agh-event\",\"data\":{\"type\":\"tool_call\"}}\n\n" +
			"data: {\"type\":\"error\",\"errorText\":\"boom\"}\n\n" +
			"data: [DONE]\n\n",
	)

	records, err := readSSERecords(stream, 0)
	if err != nil {
		t.Fatalf("readSSERecords() error = %v", err)
	}

	got := make([]string, 0, len(records))
	for _, record := range records {
		got = append(got, record.Event)
	}
	want := []string{"agent_message", "permission", "tool_call", "error", "done"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("record events = %#v, want %#v", got, want)
	}
}

func TestReadSSERecordsUntilReturnsBeforeEOF(t *testing.T) {
	t.Parallel()

	reader, writer := io.Pipe()
	releaseWriter := make(chan struct{})
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		_, _ = writer.Write([]byte("event: runtime_progress\ndata: {\"type\":\"runtime_progress\"}\n\n"))
		<-releaseWriter
		_ = writer.Close()
	}()

	records, err := readSSERecordsUntil(reader, func(record SSEEvent) bool {
		return record.Event == "runtime_progress"
	})
	close(releaseWriter)
	<-writerDone
	_ = reader.Close()
	if err != nil {
		t.Fatalf("readSSERecordsUntil() error = %v", err)
	}
	if got, want := len(records), 1; got != want {
		t.Fatalf("len(records) = %d, want %d", got, want)
	}
	if got, want := records[0].Event, "runtime_progress"; got != want {
		t.Fatalf("records[0].Event = %q, want %q", got, want)
	}
}

func TestInferSSEEventNameRecognizesAdditionalFrameTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "reasoning delta", data: `{"type":"reasoning-delta","delta":"think"}`, want: "reasoning"},
		{name: "tool output", data: `{"type":"tool-output-available","toolCallId":"tool-1"}`, want: "tool_result"},
		{name: "generic event fallback", data: `{"type":"data-agh-event","data":{}}`, want: "event"},
		{name: "unknown passthrough", data: `{"type":"finish"}`, want: "finish"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := inferSSEEventName([]byte(tt.data)); got != tt.want {
				t.Fatalf("inferSSEEventName(%s) = %q, want %q", tt.data, got, tt.want)
			}
		})
	}
}

func TestRuntimeHarnessStopFallsBackToInterruptWhenCLIStopFails(t *testing.T) {
	t.Parallel()

	cmd := exec.CommandContext(
		context.Background(),
		"sh",
		"-c",
		"trap 'exit 0' INT; while :; do sleep 1; done",
	)
	// Mirror startDaemonProcess: Stop signals the process group, so the harness
	// process must be its own group leader.
	procutil.ConfigureCommandProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("cmd.Start() error = %v", err)
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
		close(waitCh)
	}()

	harness := &RuntimeHarness{
		process: cmd,
		waitCh:  waitCh,
		CLI: &CLIClient{
			binaryPath: writeCLIScript(t, "#!/bin/sh\nexit 1\n"),
			workdir:    t.TempDir(),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := harness.Stop(ctx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if err := harness.Stop(ctx); err != nil {
		t.Fatalf("second Stop() error = %v", err)
	}
}

func TestRuntimeHarnessStartRetryHelpersRebindHTTPPortAndCleanStaleState(t *testing.T) {
	t.Parallel()

	homePaths := NewHomePaths(t)
	cfg := SeedConfig(t, homePaths, ConfigSeedOptions{HTTPPort: 22123})
	processLogPath := filepath.Join(t.TempDir(), "daemon-process.log")
	if err := os.WriteFile(
		processLogPath,
		[]byte("error: daemon: start http server: listen tcp 127.0.0.1:22123: bind: address already in use\n"),
		0o600,
	); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", processLogPath, err)
	}
	if err := os.WriteFile(homePaths.DaemonInfo, []byte(`{"pid":1}`), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", homePaths.DaemonInfo, err)
	}
	if err := os.WriteFile(cfg.Daemon.Socket, []byte("socket"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", cfg.Daemon.Socket, err)
	}

	waitCh := make(chan error, 1)
	waitCh <- errors.New("exit status 1")
	close(waitCh)

	harness := &RuntimeHarness{
		HomePaths:      homePaths,
		Config:         cfg,
		HTTPBaseURL:    fmt.Sprintf("http://%s:%d", cfg.HTTP.Host, cfg.HTTP.Port),
		processLogPath: processLogPath,
		waitCh:         waitCh,
		processExited:  true,
		processErr:     errors.New("exit status 1"),
	}

	readinessErr := fmt.Errorf("%w: %w", errDaemonExitedBeforeReadiness, errors.New("exit status 1"))
	if !harness.readinessFailureShouldRetry(readinessErr) {
		t.Fatal("readinessFailureShouldRetry() = false, want true for bind conflict")
	}
	if harness.readinessFailureShouldRetry(errors.New("daemon exited before readiness: exit status 1")) {
		t.Fatal("readinessFailureShouldRetry() = true for string-only error, want false")
	}

	if err := harness.cleanupFailedStart(testContext(t)); err != nil {
		t.Fatalf("cleanupFailedStart() error = %v", err)
	}
	for _, path := range []string{homePaths.DaemonInfo, cfg.Daemon.Socket} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Stat(%q) error = %v, want os.ErrNotExist", path, err)
		}
	}
	if harness.waitCh != nil {
		t.Fatal("cleanupFailedStart() left waitCh populated, want reset state")
	}

	previousPort := harness.Config.HTTP.Port
	if err := harness.reseedRuntimeHTTPPort(t); err != nil {
		t.Fatalf("reseedRuntimeHTTPPort() error = %v", err)
	}
	if harness.Config.HTTP.Port == previousPort {
		t.Fatalf("reseedRuntimeHTTPPort() kept HTTP port %d, want new port", previousPort)
	}
	if got, wantPrefix := harness.HTTPBaseURL, fmt.Sprintf(
		"http://%s:",
		harness.Config.HTTP.Host,
	); !strings.HasPrefix(
		got,
		wantPrefix,
	) {
		t.Fatalf("HTTPBaseURL = %q, want prefix %q", got, wantPrefix)
	}

	configContents, err := os.ReadFile(homePaths.ConfigFile)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", homePaths.ConfigFile, err)
	}
	if !strings.Contains(string(configContents), fmt.Sprintf("port = %d", harness.Config.HTTP.Port)) {
		t.Fatalf("config contents = %s, want rewritten port %d", string(configContents), harness.Config.HTTP.Port)
	}

	if err := os.WriteFile(
		processLogPath,
		[]byte(
			"error: daemon: start uds server: udsapi: listen on \"/tmp/agh.sock\": listen unix /tmp/agh.sock: bind: file exists\n",
		),
		0o600,
	); err != nil {
		t.Fatalf("os.WriteFile(%q) socket conflict error = %v", processLogPath, err)
	}
	if !harness.readinessFailureShouldRetry(readinessErr) {
		t.Fatal("readinessFailureShouldRetry() = false, want true for socket bind conflict")
	}
	if retryHTTPPort, retrySocketPath := harness.readinessFailureRetryReasons(
		readinessErr,
	); retryHTTPPort || !retrySocketPath {
		t.Fatalf(
			"readinessFailureRetryReasons(socket conflict) = (%v, %v), want (false, true)",
			retryHTTPPort,
			retrySocketPath,
		)
	}

	if err := os.WriteFile(
		processLogPath,
		[]byte(
			"error: daemon: start uds server: udsapi: listen on \"/tmp/agh.sock\": listen unix /tmp/agh.sock: bind: address already in use\n",
		),
		0o600,
	); err != nil {
		t.Fatalf("os.WriteFile(%q) socket address conflict error = %v", processLogPath, err)
	}
	if retryHTTPPort, retrySocketPath := harness.readinessFailureRetryReasons(
		readinessErr,
	); retryHTTPPort || !retrySocketPath {
		t.Fatalf(
			"readinessFailureRetryReasons(socket address conflict) = (%v, %v), want (false, true)",
			retryHTTPPort,
			retrySocketPath,
		)
	}

	previousSocket := harness.Config.Daemon.Socket
	if err := harness.reseedRuntimeSocketPath(t); err != nil {
		t.Fatalf("reseedRuntimeSocketPath() error = %v", err)
	}
	if harness.Config.Daemon.Socket == previousSocket {
		t.Fatalf("reseedRuntimeSocketPath() kept socket %q, want new path", previousSocket)
	}
	if harness.UDSClient == nil {
		t.Fatal("reseedRuntimeSocketPath() left UDSClient nil")
	}
}

func TestRuntimeHarnessPromptSessionUntilRejectsNilPredicateBeforeRequest(t *testing.T) {
	t.Parallel()

	var requested atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requested.Store(true)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	harness := &RuntimeHarness{
		UDSBaseURL: server.URL,
		UDSClient:  server.Client(),
	}

	records, err := harness.PromptSessionUntil(context.Background(), "sess-nil-predicate", "hello", nil)
	if err == nil {
		t.Fatal("PromptSessionUntil(nil predicate) error = nil, want validation error")
	}
	if records != nil {
		t.Fatalf("PromptSessionUntil(nil predicate) records = %#v, want nil", records)
	}
	if requested.Load() {
		t.Fatal("PromptSessionUntil(nil predicate) issued an HTTP request")
	}
}

func TestCLIClientRunInDirResolvesRelativePathsAgainstBaseWorkdir(t *testing.T) {
	t.Run("Should resolve relative paths against base workdir", func(t *testing.T) {
		t.Parallel()

		baseDir := t.TempDir()
		targetDir := filepath.Join(baseDir, "nested")
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%q) error = %v", targetDir, err)
		}

		client := &CLIClient{
			binaryPath: writeCLIScript(t, "#!/bin/sh\npwd\n"),
			workdir:    baseDir,
		}

		stdout, stderr, err := client.RunInDir(context.Background(), "nested", "ignored")
		if err != nil {
			t.Fatalf("RunInDir() error = %v; stderr=%s", err, strings.TrimSpace(stderr))
		}
		if got, want := strings.TrimSpace(stdout), targetDir; got != want {
			t.Fatalf("RunInDir() stdout = %q, want %q", got, want)
		}
	})
}

func TestRuntimeHelpersCoverRequestAndTimingUtilities(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/bad":
			http.Error(w, "bad request", http.StatusBadRequest)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var payload map[string]string
	if err := doJSONRequest(
		context.Background(),
		server.Client(),
		server.URL+"/ok",
		http.MethodPost,
		map[string]string{"hello": "world"},
		&payload,
	); err != nil {
		t.Fatalf("doJSONRequest(/ok) error = %v", err)
	}
	if got, want := payload["status"], "ok"; got != want {
		t.Fatalf("payload[status] = %q, want %q", got, want)
	}

	if err := doJSONRequest(
		context.Background(),
		server.Client(),
		server.URL+"/bad",
		http.MethodGet,
		nil,
		nil,
	); err == nil {
		t.Fatal("doJSONRequest(/bad) error = nil, want non-nil")
	}
	if err := doJSONRequest(context.Background(), nil, server.URL+"/ok", http.MethodGet, nil, nil); err == nil {
		t.Fatal("doJSONRequest(nil client) error = nil, want stable validation error")
	}
	if _, err := requestBody(make(chan int)); err == nil {
		t.Fatal("requestBody(chan) error = nil, want non-nil")
	}
	if got, want := ensureLeadingSlash("api/demo"), "/api/demo"; got != want {
		t.Fatalf("ensureLeadingSlash() = %q, want %q", got, want)
	}
	if got, want := ensureLeadingSlash("/api/demo"), "/api/demo"; got != want {
		t.Fatalf("ensureLeadingSlash(existing slash) = %q, want %q", got, want)
	}
	if got, want := defaultDuration(0, time.Second), time.Second; got != want {
		t.Fatalf("defaultDuration() = %s, want %s", got, want)
	}
	if got, want := defaultDuration(2*time.Second, time.Second), 2*time.Second; got != want {
		t.Fatalf("defaultDuration(non-zero) = %s, want %s", got, want)
	}
	if got, want := maxInt(1, 2), 2; got != want {
		t.Fatalf("max() = %d, want %d", got, want)
	}
	if got, want := maxInt(3, 2), 3; got != want {
		t.Fatalf("max(non-fallback) = %d, want %d", got, want)
	}
	if got, want := encodeQuery(nil), ""; got != want {
		t.Fatalf("encodeQuery(nil) = %q, want %q", got, want)
	}
}

func TestRuntimeHarnessCaptureCLIOutputWritesTransportArtifactsAndManifest(t *testing.T) {
	t.Parallel()

	homePaths := NewHomePaths(t)
	cfg := SeedConfig(t, homePaths, ConfigSeedOptions{HTTPPort: 23123})
	harness := &RuntimeHarness{
		HomePaths:      homePaths,
		Config:         cfg,
		Artifacts:      NewArtifactCollector(t),
		WorkspaceRoot:  "/workspace",
		HTTPBaseURL:    fmt.Sprintf("http://%s:%d", cfg.HTTP.Host, cfg.HTTP.Port),
		UDSBaseURL:     "http://unix",
		processLogPath: filepath.Join(t.TempDir(), "daemon-process.log"),
		CLI: &CLIClient{
			binaryPath: "/tmp/agh",
			workdir:    "/repo",
		},
	}

	cliPath, err := harness.CaptureCLIOutput(
		"runtime status",
		[]string{"status", "-o", "json"},
		`{"status":"running"}`,
		"",
		nil,
	)
	if err != nil {
		t.Fatalf("CaptureCLIOutput() error = %v", err)
	}
	if got, want := filepath.Base(cliPath), "runtime-status.json"; got != want {
		t.Fatalf("filepath.Base(cliPath) = %q, want %q", got, want)
	}

	httpPath, err := harness.CaptureTransportOutput("http status", TransportOutputArtifact{
		Transport:  "http",
		Method:     http.MethodGet,
		URL:        "/api/status",
		StatusCode: http.StatusOK,
	})
	if err != nil {
		t.Fatalf("CaptureTransportOutput() error = %v", err)
	}
	if got, want := filepath.Base(httpPath), "http-status.json"; got != want {
		t.Fatalf("filepath.Base(httpPath) = %q, want %q", got, want)
	}

	runtimeManifest, err := harness.RuntimeManifest()
	if err != nil {
		t.Fatalf("RuntimeManifest() error = %v", err)
	}
	if got, want := runtimeManifest.Transport.CLIBinary, "/tmp/agh"; got != want {
		t.Fatalf("runtimeManifest.Transport.CLIBinary = %q, want %q", got, want)
	}
	if got, want := runtimeManifest.Transport.CLIWorkdir, "/repo"; got != want {
		t.Fatalf("runtimeManifest.Transport.CLIWorkdir = %q, want %q", got, want)
	}

	manifestBytes, err := os.ReadFile(harness.RuntimeManifestPath())
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", harness.RuntimeManifestPath(), err)
	}
	if !strings.Contains(string(manifestBytes), "\"transport_outputs\"") {
		t.Fatalf("runtime manifest = %s, want transport_outputs artifact entry", string(manifestBytes))
	}
}

func TestRuntimeHarnessNilGuardsSurfaceStableErrors(t *testing.T) {
	t.Parallel()

	var nilHarness *RuntimeHarness
	if got := nilHarness.RuntimeManifestPath(); got != "" {
		t.Fatalf("nil RuntimeManifestPath() = %q, want empty", got)
	}
	if err := nilHarness.Stop(testContext(t)); err != nil {
		t.Fatalf("nil Stop() error = %v, want nil", err)
	}
	if _, err := nilHarness.CaptureTransportOutput("runtime status", TransportOutputArtifact{}); err == nil {
		t.Fatal("nil CaptureTransportOutput() error = nil, want failure")
	}

	harness := &RuntimeHarness{}
	if _, err := harness.readProcessLog(); err == nil {
		t.Fatal("readProcessLog() error = nil, want missing path failure")
	}
}

func writeCLIScript(t testing.TB, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "agh-cli-script.sh")
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
	return path
}

func TestHelperUtilitiesCoverSortingAndSanitizing(t *testing.T) {
	t.Parallel()

	values := []string{"zeta", "alpha", "mid"}
	sortStrings(values)
	if got, want := values, []string{"alpha", "mid", "zeta"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sortStrings() = %#v, want %#v", got, want)
	}

	if got, want := sanitizePathComponent("  Hello, World!  "), "hello-world"; got != want {
		t.Fatalf("sanitizePathComponent() = %q, want %q", got, want)
	}
	if got, want := sanitizePathComponent(""), "run"; got != want {
		t.Fatalf("sanitizePathComponent(blank) = %q, want %q", got, want)
	}
}

func TestCaptureNetworkAuditMissingFileIsNoop(t *testing.T) {
	t.Parallel()

	harness := &RuntimeHarness{
		HomePaths: aghconfig.HomePaths{
			NetworkAuditFile: filepath.Join(t.TempDir(), "missing.audit"),
		},
		Artifacts: NewArtifactCollector(t),
	}
	if err := harness.CaptureNetworkAudit(); err != nil {
		t.Fatalf("CaptureNetworkAudit() error = %v", err)
	}
	if got := len(harness.Artifacts.Manifest().Artifacts); got != 0 {
		t.Fatalf("len(artifacts) = %d, want 0", got)
	}
}

func TestRuntimeHelpersCoverCLIEnvAndRepoUtilities(t *testing.T) {
	t.Parallel()

	homePaths := NewHomePaths(t)
	binaryPath := writeCLIScript(t, "#!/bin/sh\nexit 0\n")
	baseEnv := []string{"PATH=/usr/bin", "HOME=/tmp/demo"}

	withCLI, err := withRuntimeCLIEnv(homePaths, baseEnv, binaryPath)
	if err != nil {
		t.Fatalf("withRuntimeCLIEnv() error = %v", err)
	}

	shimPath := lookupEnvValue(withCLI, "AGH_E2E_CLI_BIN")
	if strings.TrimSpace(shimPath) == "" {
		t.Fatal("AGH_E2E_CLI_BIN = empty, want installed runtime shim")
	}
	if _, err := os.Stat(shimPath); err != nil {
		t.Fatalf("os.Stat(%q) error = %v", shimPath, err)
	}
	if got, want := filepath.Dir(shimPath), filepath.Join(homePaths.HomeDir, "bin"); got != want {
		t.Fatalf("filepath.Dir(shimPath) = %q, want %q", got, want)
	}
	if got := lookupEnvValue(withCLI, "PATH"); !strings.HasPrefix(got, filepath.Dir(shimPath)) {
		t.Fatalf("PATH = %q, want shim directory prefix", got)
	}
	if got, want := lookupEnvValue(withCLI, "HOME"), "/tmp/demo"; got != want {
		t.Fatalf("lookupEnvValue(HOME) = %q, want %q", got, want)
	}

	unchanged, err := withRuntimeCLIEnv(homePaths, baseEnv, "")
	if err != nil {
		t.Fatalf("withRuntimeCLIEnv(blank) error = %v", err)
	}
	if !reflect.DeepEqual(unchanged, baseEnv) {
		t.Fatalf("withRuntimeCLIEnv(blank) = %#v, want %#v", unchanged, baseEnv)
	}

	if got, want := setEnvValue(
		baseEnv,
		"HOME",
		"/tmp/updated",
	), []string{
		"PATH=/usr/bin",
		"HOME=/tmp/updated",
	}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("setEnvValue(update) = %#v, want %#v", got, want)
	}
	if got, want := setEnvValue(
		baseEnv,
		"NEW_VAR",
		"present",
	), []string{
		"PATH=/usr/bin",
		"HOME=/tmp/demo",
		"NEW_VAR=present",
	}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("setEnvValue(append) = %#v, want %#v", got, want)
	}
	if got, want := setEnvValue(baseEnv, "", "ignored"), baseEnv; !reflect.DeepEqual(got, want) {
		t.Fatalf("setEnvValue(blank key) = %#v, want %#v", got, want)
	}
	if got := lookupEnvValue(baseEnv, "MISSING"); got != "" {
		t.Fatalf("lookupEnvValue(MISSING) = %q, want empty", got)
	}
	if got, want := prependPath("", "/usr/bin"), "/usr/bin"; got != want {
		t.Fatalf("prependPath(blank prefix) = %q, want %q", got, want)
	}
	if got, want := prependPath("/tmp/bin", ""), "/tmp/bin"; got != want {
		t.Fatalf("prependPath(blank current) = %q, want %q", got, want)
	}

	root := mustRepoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("os.Stat(go.mod under %q) error = %v", root, err)
	}

	if got := cloneMockAgentRegistrations(nil); got != nil {
		t.Fatalf("cloneMockAgentRegistrations(nil) = %#v, want nil", got)
	}
	original := map[string]acpmock.Registration{
		"mock-coder": {AgentName: "mock-coder"},
	}
	cloned := cloneMockAgentRegistrations(original)
	original["mock-coder"] = acpmock.Registration{AgentName: "changed"}
	if got, want := cloned["mock-coder"].AgentName, "mock-coder"; got != want {
		t.Fatalf("cloned[mock-coder].AgentName = %q, want %q", got, want)
	}
}

func TestBuildAGHBinaryProducesReusableExecutable(t *testing.T) {
	path := buildAGHBinary(t)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("os.Stat(%q) error = %v", path, err)
	}

	if got, want := buildAGHBinary(t), path; got != want {
		t.Fatalf("second buildAGHBinary() = %q, want %q", got, want)
	}
}

func TestBuildAGHBinaryHonorsSandboxOverride(t *testing.T) {
	override := filepath.Join(t.TempDir(), "agh-custom")
	if err := os.WriteFile(override, []byte("fake"), 0o755); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", override, err)
	}

	t.Setenv(daemonBinaryEnvVar, override)
	if got, want := buildAGHBinary(t), override; got != want {
		t.Fatalf("buildAGHBinary() with env override = %q, want %q", got, want)
	}
}
