//go:build contentionload && !windows

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/execabs"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	"github.com/compozy/compozy/internal/transcript"
)

const contentionAgent = "contention-stream"
const contentionModel = "contention-model"
const contentionEnd = "contention stream completed"

// Suite: real runtime HTTP/SSE/UDS/CLI load (opt-in CI acceptance).
// Invariant: thirty active streams preserve interactive reads/writes and accepted
// transcript content across a clean daemon restart, without SQLite exhaustion.
// Boundary IN: public session creation/prompts through a real isolated daemon.
// Boundary OUT: HTTP reads, CLI status/Observe, reopened transcripts and daemon logs.
// The ACP peer is scripted transport load; this does not claim live-provider QA.
func TestSQLiteContentionDaemonLoad(t *testing.T) {
	t.Run("Should serve thirty streams and preserve accepted content after restart", func(t *testing.T) {
		data, err := os.ReadFile(os.Getenv("COMPOZY_QA_BOOTSTRAP_MANIFEST"))
		if err != nil {
			t.Fatal(err)
		}
		var manifest struct {
			Env map[string]string `json:"env"`
		}
		if err = json.Unmarshal(data, &manifest); err != nil {
			t.Fatal(err)
		}
		env := manifest.Env
		output := env["QA_OUTPUT_PATH"]
		paths, err := config.ResolveHomePathsFrom(env["COMPOZY_HOME"])
		if err != nil {
			t.Fatal(err)
		}
		port, err := strconv.Atoi(env["COMPOZY_HTTP_PORT"])
		if err != nil {
			t.Fatal(err)
		}
		fixture := filepath.Join(output, "stream-fixture.json")
		writeContentionFixture(t, fixture)
		opts := &RuntimeHarnessOptions{
			BinaryPath: os.Getenv(daemonBinaryEnvVar),
			HomePaths:  paths,
			ConfigSeed: ConfigSeedOptions{
				HTTPPort:     port,
				SocketPath:   env["COMPOZY_UDS_PATH"],
				DefaultAgent: contentionAgent,
			},
			Workspace: WorkspaceSeedOptions{Root: env["RUNTIME_WORKSPACE_PATH"]},
			MockAgents: []MockAgentSpec{
				{
					FixturePath:     fixture,
					FixtureAgent:    contentionAgent,
					AgentName:       contentionAgent,
					DiagnosticsPath: filepath.Join(output, "acp-diagnostics.jsonl"),
				},
			},
			Env: map[string]string{"CODEX_HOME": env["PROVIDER_CODEX_HOME"]},
		}
		if opts.BinaryPath == "" {
			t.Fatal("prebuilt daemon binary is required")
		}
		layout := prepareRuntimeLayout(t, opts)
		// ConfigSeed writes a test config; restore the bootstrap's no-account-polling policy before launch.
		file, err := os.OpenFile(paths.ConfigFile, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, writeErr := file.WriteString("\n[provider_usage]\nenabled = false\n")
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			t.Fatalf("disable provider usage: %v / %v", writeErr, closeErr)
		}
		layout.Env = runtimeEnv(paths, env["PROVIDER_HOME"], opts.Env)
		layout.Env, err = withRuntimeCLIEnv(paths, layout.Env, opts.BinaryPath)
		if err != nil {
			t.Fatal(err)
		}
		h := startContentionDaemon(t, &layout, opts.BinaryPath, output, "initial")
		ids := createContentionSessions(t, h)
		stats := exerciseContentionStreams(t, h, ids, output)
		before := captureContentionSessions(t, h, ids, output, "before-restart")
		captureContentionCLI(t, h, output, "before-restart")
		stopContentionDaemon(t, h)
		assertContentionLogs(t, output, paths.LogsDir, "initial")
		resumed := startContentionDaemon(t, &layout, opts.BinaryPath, output, "reopened")
		after := captureContentionSessions(t, resumed, ids, output, "after-restart")
		for id, text := range before {
			if after[id] != text {
				t.Errorf("session %s transcript changed across restart", id)
			}
		}
		captureContentionCLI(t, resumed, output, "after-restart")
		stopContentionDaemon(t, resumed)
		assertContentionLogs(t, output, paths.LogsDir, "reopened")
		if t.Failed() {
			return
		}
		writeContentionJSON(t, filepath.Join(output, "daemon-result.json"), map[string]any{
			"sessions": ids, "completed_streams": len(ids), "transcripts_preserved": len(after), "latency_ms": stats,
			"provider": "scripted ACP transport; no live provider credentials", "verdict": "PASS",
		})
	})
}

func writeContentionFixture(t *testing.T, path string) {
	t.Helper()
	steps := make([]acpmock.Step, 0, 1201)
	for range 600 {
		steps = append(
			steps,
			acpmock.Step{Kind: acpmock.StepKindAssistant, Text: "contention chunk "},
			acpmock.Step{
				Kind:          acpmock.StepKindDriverControl,
				DriverControl: &acpmock.DriverControlStep{Action: acpmock.DriverControlDelay, DelayMS: 50},
			},
		)
	}
	steps = append(steps, acpmock.Step{Kind: acpmock.StepKindAssistant, Text: contentionEnd})
	writeContentionJSON(t, path, acpmock.Fixture{Version: acpmock.FixtureVersion, Agents: []acpmock.AgentFixture{{
		Name: contentionAgent, Provider: "mock", Model: contentionModel,
		ConfigOptions: []acpmock.SessionConfigOptionFixture{{
			ID: "model", Name: "Model", Category: "model", Current: contentionModel,
			Values: []acpmock.SessionConfigOptionValueFixture{{Value: contentionModel, Label: "Contention model"}},
		}},
		Turns: []acpmock.TurnFixture{{
			Match: acpmock.TurnMatch{UserTextContains: "stream"}, Steps: steps,
		}},
	}}})
}

func startContentionDaemon(t *testing.T, layout *runtimeLayout, binary, output, phase string) *RuntimeHarness {
	t.Helper()
	h := newRuntimeHarness(t, layout, binary)
	h.processLogPath = filepath.Join(output, "daemon-"+phase+".log")
	startDaemonProcess(t, h, layout.Env)
	t.Cleanup(func() { stopContentionDaemon(t, h) })
	register := filepath.Join(
		mustRepoRoot(t),
		".agents",
		"skills",
		"eng",
		"eng-qa-bootstrap",
		"scripts",
		"qa_process_identity.py",
	)
	cmd := execabs.CommandContext(t.Context(), "python3", register, "--pid", strconv.Itoa(h.process.Process.Pid),
		"--file", filepath.Join(output, "qa/pids/daemon-"+phase+".pid"))
	if data, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("register daemon PID: %v: %s", err, data)
	}
	readyCtx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	if err := h.waitForReady(readyCtx, 50*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	ws, err := h.ResolveWorkspace(readyCtx, layout.WorkspaceRoot)
	if err != nil {
		t.Fatal(err)
	}
	h.WorkspaceID = ws.ID
	manifest, err := h.WriteRuntimeManifest()
	if err != nil {
		t.Fatal(err)
	}
	writeContentionJSON(t, filepath.Join(output, "runtime-"+phase+".json"), manifest)
	return h
}

func stopContentionDaemon(t *testing.T, h *RuntimeHarness) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := h.Stop(ctx); err != nil {
		t.Errorf("clean daemon stop: %v", err)
	}
	if _, err := h.pollExit(); err != nil {
		t.Errorf("daemon did not exit cleanly: %v", err)
	}
}

func createContentionSessions(t *testing.T, h *RuntimeHarness) []string {
	t.Helper()
	ids := make([]string, 0, 30)
	for index := range 30 {
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		created, err := h.CreateSession(ctx, contract.CreateSessionRequest{AgentName: contentionAgent,
			Name: fmt.Sprintf("Contention %02d", index), WorkspacePath: h.WorkspaceRoot})
		if err == nil {
			_, err = h.WaitForSessionActive(ctx, created.ID)
		}
		cancel()
		if err != nil {
			t.Fatalf("create session %d: %v", index, err)
		}
		ids = append(ids, created.ID)
	}
	return ids
}

type contentionRouteSamples struct {
	mu       sync.Mutex
	values   map[string][]float64
	failures []string
}

func (s *contentionRouteSamples) request(name string, fn func(context.Context) error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	started := time.Now()
	err := fn(ctx)
	elapsed := float64(time.Since(started)) / float64(time.Millisecond)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[name] = append(s.values[name], elapsed)
	if err != nil {
		s.failures = append(s.failures, name+": "+err.Error())
	}
}

func exerciseContentionStreams(t *testing.T, h *RuntimeHarness, ids []string, output string) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	first := make(chan string, len(ids))
	errorsCh := make(chan error, len(ids))
	var prompts sync.WaitGroup
	defer func() {
		cancel()
		prompts.Wait()
	}()
	for _, id := range ids {
		prompts.Go(func() {
			var once sync.Once
			events, err := h.PromptSessionHTTPWithEvents(ctx, id, "stream", func(event SSEEvent) error {
				if strings.Contains(string(event.Data), "contention chunk") {
					once.Do(func() { first <- id })
				}
				return nil
			})
			if err == nil && len(events) == 0 {
				err = fmt.Errorf("empty prompt stream for %s", id)
			}
			errorsCh <- err
		})
	}
	firstChunkAt := time.Time{}
	for range ids {
		select {
		case <-first:
			if firstChunkAt.IsZero() {
				firstChunkAt = time.Now()
			}
		case err := <-errorsCh:
			t.Fatalf("stream ended before all thirty sessions started: %v", err)
		case <-ctx.Done():
			t.Fatal("not all thirty sessions produced a chunk: ", ctx.Err())
		}
	}
	if time.Since(firstChunkAt) > 5*time.Second {
		t.Fatal("streams did not overlap for the required 20-second read window")
	}
	samples := &contentionRouteSamples{values: make(map[string][]float64)}
	readContentionRoutes(h, ids, samples, 20*time.Second)
	prompts.Wait()
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Error("stream failed: ", err)
		}
	}
	writeContentionJSON(t, filepath.Join(output, "http-samples.json"), samples.values)
	if len(samples.failures) > 0 {
		t.Fatalf("interactive route failures: %v", samples.failures)
	}
	result := make(map[string]any)
	for route, values := range samples.values {
		slices.Sort(values)
		result[route] = map[string]any{
			"count": len(values),
			"p99":   values[int(float64(len(values)-1)*.99)],
			"max":   values[len(values)-1],
		}
	}
	return result
}

func readContentionRoutes(h *RuntimeHarness, ids []string, samples *contentionRouteSamples, duration time.Duration) {
	until := time.Now().Add(duration)
	var readers sync.WaitGroup
	for _, id := range ids {
		readers.Go(func() {
			ticker := time.NewTicker(200 * time.Millisecond)
			defer ticker.Stop()
			for time.Now().Before(until) {
				samples.request("transcript", func(ctx context.Context) error {
					var response contract.SessionTranscriptResponse
					return h.HTTPJSON(
						ctx,
						http.MethodGet,
						"/api/workspaces/"+h.WorkspaceID+"/sessions/"+id+"/transcript?limit=60",
						nil,
						&response,
					)
				})
				<-ticker.C
			}
		})
	}
	readers.Go(func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for n := 0; time.Now().Before(until); n++ {
			samples.request("catalog", func(ctx context.Context) error {
				var response contract.SessionsResponse
				err := h.HTTPJSON(
					ctx,
					http.MethodGet,
					"/api/workspaces/"+h.WorkspaceID+"/sessions?limit=30",
					nil,
					&response,
				)
				if err == nil && len(response.Sessions) != len(ids) {
					return fmt.Errorf("catalog has %d sessions, want %d", len(response.Sessions), len(ids))
				}
				return err
			})
			if n%5 == 0 {
				samples.request("rename", func(ctx context.Context) error {
					var response contract.SessionResponse
					return h.HTTPJSON(
						ctx,
						http.MethodPatch,
						"/api/workspaces/"+h.WorkspaceID+"/sessions/"+ids[0],
						contract.RenameSessionRequest{Name: fmt.Sprintf("Contention renamed %d", n)},
						&response,
					)
				})
			}
			<-ticker.C
		}
	})
	readers.Wait()
}

func captureContentionSessions(t *testing.T, h *RuntimeHarness, ids []string, output, phase string) map[string]string {
	t.Helper()
	texts := make(map[string]string, len(ids))
	for _, id := range ids {
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		page, err := h.SessionTranscript(ctx, id)
		var health contract.SessionHealthResponse
		if err == nil {
			err = h.HTTPJSON(
				ctx,
				http.MethodGet,
				"/api/workspaces/"+h.WorkspaceID+"/sessions/"+id+"/health",
				nil,
				&health,
			)
		}
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		var text strings.Builder
		for _, message := range transcript.MessagesFromEntries(page.Entries) {
			text.WriteString(transcript.UIMessageText(message))
		}
		if !strings.Contains(text.String(), contentionEnd) {
			t.Fatalf("session %s lacks completed stream", id)
		}
		if health.Health.SessionID != id || health.Health.UpdatedAt.IsZero() {
			t.Fatalf("missing durable health for %s: %#v", id, health)
		}
		texts[id] = text.String()
		writeContentionJSON(
			t,
			filepath.Join(output, phase+"-"+id+".json"),
			map[string]any{"transcript": page, "health": health},
		)
	}
	return texts
}

func captureContentionCLI(t *testing.T, h *RuntimeHarness, output, phase string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	for _, args := range [][]string{{"status", "-o", "json"}, {"observe", "overview", "-o", "json"}} {
		stdout, stderr, err := h.CLI.Run(ctx, args...)
		if err != nil {
			t.Fatalf("CLI %v: %v: %s", args, err, stderr)
		}
		if !json.Valid([]byte(stdout)) {
			t.Fatalf("invalid CLI JSON: %s", stdout)
		}
		if err = os.WriteFile(filepath.Join(output, phase+"-"+args[0]+".json"), []byte(stdout), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func assertContentionLogs(t *testing.T, output, logsDir, phase string) {
	t.Helper()
	assertContentionLogFile(t, filepath.Join(output, "daemon-"+phase+".log"))
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		source := filepath.Join(logsDir, entry.Name())
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(output, phase+"-"+entry.Name())
		if err = os.WriteFile(target, data, 0600); err != nil {
			t.Fatal(err)
		}
		assertContentionLogFile(t, target)
	}
}

func assertContentionLogFile(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		// Maintenance errors must not hide behind otherwise successful requests.
		if strings.Contains(line, "contention exhausted bounded retry") ||
			strings.Contains(line, "held for at least one second") ||
			strings.Contains(line, "sqlite passive checkpoint failed") ||
			strings.Contains(line, "passive session wal checkpoint failed") ||
			strings.Contains(line, "truncate session wal failed") ||
			strings.Contains(line, "SQLITE_BUSY") {
			t.Errorf("SQLite contention in daemon log: %s", line)
		}
	}
}

func writeContentionJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
