//go:build !windows

package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providers"
	"github.com/compozy/compozy/internal/session/sessionid"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/telemetry"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	"github.com/compozy/compozy/internal/testutil/otlptest"
	"github.com/compozy/compozy/internal/toolmeta"
	"go.opentelemetry.io/otel/attribute"
	colmetricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

const (
	footprintAgent  = "footprint"
	footprintPrompt = "footprint turn"
	footprintChunks = 200
)

// BenchmarkTurnTelemetryFootprint measures a representative turn with export disabled and enabled at 100%:
//
//	go test ./internal/session -run '^$' -bench BenchmarkTurnTelemetryFootprint -benchmem
//
// Each turn runs through the real ACP driver and acpmock on a warm session: 200 streamed chunks, one hosted tool
// call, and a usage event. Enabled mode exports to a separate-process OTLP/HTTP receiver and installs the daemon's
// store write observer. Besides ns/turn it reports
// cpu_ns/turn (this process's user+sys CPU from getrusage around the measured loop, the final export included),
// otlp_bytes/turn (serialized trace request bytes per turn plus one full metric push), and spans/turn.
func BenchmarkTurnTelemetryFootprint(b *testing.B) {
	driverPath, err := acpmock.DefaultDriverPath()
	if err != nil {
		b.Fatalf("acpmock.DefaultDriverPath() error = %v", err)
	}
	fixture := writeFootprintFixture(b)
	for _, mode := range []struct {
		name   string
		export bool
	}{
		{name: "disabled"},
		{name: "enabled_100", export: true},
	} {
		b.Run(mode.name, func(b *testing.B) {
			var collector *otlptest.Collector
			var receiver *footprintReceiver
			if mode.export {
				receiver = startFootprintReceiver(b)
				collector = receiver.collector
				collector.ExportWithMetrics(b, 1, footprintRegistries())
				// The daemon installs this observer whenever export starts; default slow-write threshold.
				defaults := compozyconfig.DefaultWithHome(compozyconfig.HomePaths{})
				slowDBWriteMS := defaults.Observability.OTel.SlowDBWriteMS
				store.SetWriteObserver(func(ctx context.Context, write store.WriteObservation) {
					telemetry.RecordDBWrite(ctx, telemetry.DBWrite(write), slowDBWriteMS)
				})
				b.Cleanup(func() { store.SetWriteObserver(nil) })
			}
			h, sessionID := startFootprintSession(b, driverPath, fixture)
			runFootprintTurn(b, h, sessionID) // warm-up: runtime start and the first turn stay out of the measurement
			var traceBytes, spans int
			if collector != nil {
				collector.Flush(b)
				warmup := receiver.snapshot(b)
				traceBytes, spans = warmup.TraceBytes, warmup.Spans
			}

			before := processCPU(b)
			start := time.Now()
			turns := 0
			for b.Loop() {
				runFootprintTurn(b, h, sessionID)
				turns++
			}
			if collector != nil {
				collector.Flush(b)
				collector.FlushMetrics(b)
			}
			cpu := processCPU(b) - before
			elapsed := time.Since(start)

			b.ReportMetric(float64(elapsed.Nanoseconds())/float64(turns), "ns/turn")
			b.ReportMetric(float64(cpu.Nanoseconds())/float64(turns), "cpu_ns/turn")
			otlpBytes, turnSpans := 0.0, 0.0
			if collector != nil {
				// Read receiver accounting only after stopping the process CPU and elapsed measurements.
				final := receiver.snapshot(b)
				traces := final.TraceBytes - traceBytes
				if traces == 0 || final.MetricPushes == 0 {
					b.Fatalf("receiver got %d trace bytes and %d metric pushes, want both", traces, final.MetricPushes)
				}
				otlpBytes = float64(traces)/float64(turns) + float64(final.LastMetricBytes)
				turnSpans = float64(final.Spans-spans) / float64(turns)
			}
			b.ReportMetric(otlpBytes, "otlp_bytes/turn")
			b.ReportMetric(turnSpans, "spans/turn")
		})
	}
}

type footprintSnapshot struct {
	TraceBytes      int
	Spans           int
	MetricPushes    int
	LastMetricBytes int
}

type footprintReceiver struct {
	collector *otlptest.Collector
	requests  *json.Encoder
	responses *json.Decoder
	cancel    context.CancelFunc
}

// TestTurnTelemetryReceiver is also the re-exec entry point, selected by TestMain before store seeding.
func TestTurnTelemetryReceiver(t *testing.T) {
	if os.Getenv(footprintReceiverEnv) == "1" {
		serveFootprintReceiver(t)
		return
	}
	t.Run("Should receive trace and metric exports in a separate process", func(t *testing.T) {
		t.Parallel()
		receiver := startFootprintReceiver(t)
		if got := receiver.snapshot(t); got != (footprintSnapshot{}) {
			t.Fatalf("initial receiver snapshot = %+v, want empty", got)
		}
		traces := postFootprintExport(t, receiver.collector.URL+otlptest.TracesPath,
			&coltracepb.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{
				ScopeSpans: []*tracepb.ScopeSpans{{Spans: []*tracepb.Span{{Name: "turn"}}}},
			}}})
		metrics := postFootprintExport(t, receiver.collector.URL+otlptest.MetricsPath,
			&colmetricpb.ExportMetricsServiceRequest{ResourceMetrics: []*metricpb.ResourceMetrics{{
				ScopeMetrics: []*metricpb.ScopeMetrics{{Metrics: []*metricpb.Metric{{Name: "turns"}}}},
			}}})
		want := footprintSnapshot{TraceBytes: traces, Spans: 1, MetricPushes: 1, LastMetricBytes: metrics}
		if got := receiver.snapshot(t); got != want {
			t.Fatalf("receiver snapshot = %+v, want %+v", got, want)
		}
	})
}

func postFootprintExport(t *testing.T, endpoint string, message proto.Message) int {
	t.Helper()
	body, err := proto.Marshal(message)
	if err != nil {
		t.Fatalf("marshal OTLP export: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create OTLP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-protobuf")
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		t.Fatalf("post OTLP export: %v", err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Errorf("close OTLP response: %v", err)
		}
	}()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("OTLP response status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	return len(body)
}

func serveFootprintReceiver(t *testing.T) {
	t.Helper()
	collector := otlptest.NewCollector(t)
	responses := json.NewEncoder(os.Stdout)
	if err := responses.Encode(collector.URL); err != nil {
		t.Fatalf("write OTLP receiver endpoint: %v", err)
	}
	requests := json.NewDecoder(os.Stdin)
	for {
		var request string
		if err := requests.Decode(&request); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			t.Fatalf("read OTLP receiver request: %v", err)
		}
		if request != "snapshot" {
			t.Fatalf("unknown OTLP receiver request %q", request)
		}
		metrics := collector.Payloads(otlptest.MetricsPath)
		snapshot := footprintSnapshot{
			TraceBytes: payloadBytes(collector.Payloads(otlptest.TracesPath)),
			Spans:      len(collector.Spans()), MetricPushes: len(metrics),
		}
		if len(metrics) > 0 {
			snapshot.LastMetricBytes = len(metrics[len(metrics)-1])
		}
		if err := responses.Encode(snapshot); err != nil {
			t.Fatalf("write OTLP receiver snapshot: %v", err)
		}
	}
}

func startFootprintReceiver(t testing.TB) *footprintReceiver {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatalf("find OTLP receiver test binary: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	command := exec.CommandContext(ctx, binary, "-test.run=^TestTurnTelemetryReceiver$")
	command.Env = append(os.Environ(), footprintReceiverEnv+"=1")
	command.Stderr = os.Stderr
	command.WaitDelay = 5 * time.Second
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatalf("create OTLP receiver input: %v", err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		if closeErr := input.Close(); closeErr != nil {
			t.Errorf("close OTLP receiver input: %v", closeErr)
		}
		t.Fatalf("create OTLP receiver output: %v", err)
	}
	if err := command.Start(); err != nil {
		t.Fatalf("start OTLP receiver: %v", err)
	}
	t.Cleanup(func() {
		// EOF stops the receiver even when its parent exits; cancellation bounds a stuck shutdown.
		timer := time.AfterFunc(5*time.Second, cancel)
		defer timer.Stop()
		if err := input.Close(); err != nil {
			t.Errorf("close OTLP receiver input: %v", err)
		}
		if err := command.Wait(); err != nil {
			t.Errorf("wait for OTLP receiver: %v", err)
		}
	})
	receiver := &footprintReceiver{
		requests: json.NewEncoder(input), responses: json.NewDecoder(output), cancel: cancel,
	}
	timer := time.AfterFunc(10*time.Second, cancel)
	defer timer.Stop()
	var endpoint string
	if err := receiver.responses.Decode(&endpoint); err != nil {
		t.Fatalf("read OTLP receiver endpoint: %v", err)
	}
	// Reuse the exporter/flush helpers without starting a receiver in this process.
	receiver.collector = &otlptest.Collector{URL: endpoint}
	return receiver
}

func (r *footprintReceiver) snapshot(t testing.TB) footprintSnapshot {
	t.Helper()
	timer := time.AfterFunc(10*time.Second, r.cancel)
	defer timer.Stop()
	if err := r.requests.Encode("snapshot"); err != nil {
		t.Fatalf("request OTLP receiver snapshot: %v", err)
	}
	var snapshot footprintSnapshot
	if err := r.responses.Decode(&snapshot); err != nil {
		t.Fatalf("read OTLP receiver snapshot: %v", err)
	}
	return snapshot
}

func writeFootprintFixture(b *testing.B) string {
	b.Helper()
	chunks := make([]string, footprintChunks)
	for i := range chunks {
		chunks[i] = "chunk "
	}
	fixture := map[string]any{
		"version": acpmock.FixtureVersion,
		"agents": []map[string]any{{
			"name": footprintAgent, "provider": acpmock.ProviderName, "permissions": "approve-all",
			"turns": []map[string]any{{
				"match": map[string]any{"turn_source": "user", "user_text": footprintPrompt},
				"usage": map[string]any{"input_tokens": 1200, "output_tokens": 400},
				"steps": []map[string]any{
					{"kind": "assistant", "chunks": chunks},
					{
						"kind": "native_tool_call", "tool_id": hostedToolsHelperTool, "tool_call_id": "list-1",
						"raw_input": map[string]any{},
					},
					{"kind": "assistant", "text": "done"},
				},
			}},
		}},
	}
	raw, err := json.Marshal(fixture)
	if err != nil {
		b.Fatalf("encode footprint fixture: %v", err)
	}
	path := filepath.Join(b.TempDir(), "footprint_fixture.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		b.Fatalf("write footprint fixture: %v", err)
	}
	return path
}

// startFootprintSession creates a session on the acpmock agent with the test binary standing in for the hosted
// tools server.
func startFootprintSession(b *testing.B, driverPath, fixture string) (*harness, string) {
	b.Helper()
	hosted := &recordingHostedMCPLauncher{server: compozyconfig.MCPServer{
		Name:      toolmeta.HostedServerName,
		Transport: compozyconfig.MCPServerTransportStdio,
		Command:   os.Args[0],
		Env:       map[string]string{hostedToolsHelperEnv: "1"},
	}}
	h := newHarness(b)
	h.cfg.Providers[acpmock.ProviderName] = acpmock.ProviderConfig(driverPath)
	resolved, err := h.resolver.Resolve(b.Context(), h.workspaceID)
	if err != nil {
		b.Fatalf("Resolve(%q) error = %v", h.workspaceID, err)
	}
	resolved.Config = h.cfg
	resolved.Agents = []compozyconfig.AgentDef{{
		Name:     footprintAgent,
		Provider: acpmock.ProviderName,
		Command:  acpmock.BuildCommand(driverPath, fixture, footprintAgent, ""),
		Prompt:   "You are a deterministic benchmark agent.",
	}}
	h.resolver.upsert(&resolved)
	driver := NewACPDriverAdapter(acp.New(
		acp.WithProviderPreStarter(providers.NewPreStarter()),
		acp.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
	))
	if err := h.manager.Shutdown(b.Context()); err != nil {
		b.Fatalf("Manager.Shutdown() first manager error = %v", err)
	}
	h.manager = newManagerWithHarness(b, h, WithDriver(driver), WithHostedMCPLauncher(hosted),
		WithNotifier(discardNotifier{}))
	b.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := h.manager.Shutdown(ctx); err != nil {
			b.Errorf("Manager.Shutdown() error = %v", err)
		}
	})
	session, err := h.manager.Create(b.Context(), new(CreateOpts{AgentName: footprintAgent, Workspace: h.workspaceID}))
	if err != nil {
		b.Fatalf("Create() error = %v", err)
	}
	return h, session.ID
}

// runFootprintTurn prompts once and checks the turn streamed every chunk and completed the hosted tool call.
func runFootprintTurn(b *testing.B, h *harness, sessionID string) {
	b.Helper()
	events, err := h.manager.Prompt(b.Context(), sessionID, footprintPrompt)
	if err != nil {
		b.Fatalf("Prompt() error = %v", err)
	}
	var text strings.Builder
	toolDone := false
	for event := range events {
		switch event.Type {
		case acp.EventTypeAgentMessage:
			text.WriteString(event.Text)
		case acp.EventTypeToolResult:
			toolDone = true
		}
	}
	if got := strings.Count(text.String(), "chunk "); got != footprintChunks || !toolDone {
		b.Fatalf("turn streamed %d chunks, tool result %v; want %d chunks and the tool result",
			got, toolDone, footprintChunks)
	}
}

// discardNotifier drops fan-out notifications; the daemon's SSE fan-out is outside the measured turn.
type discardNotifier struct{}

func (discardNotifier) OnSessionCreated(context.Context, *Session) {}
func (discardNotifier) OnSessionStopped(context.Context, *Session) {}
func (discardNotifier) OnAgentEvent(context.Context, string, any)  {}

func footprintRegistries() telemetry.Registries {
	return telemetry.Registries{
		ACPMethods: acp.TracedMethods(),
		IDs: map[attribute.Key]func(string) bool{
			telemetry.AttrSessionID: sessionid.IsSessionID,
			telemetry.AttrTurnID:    sessionid.IsTurnID,
			telemetry.AttrRunID:     sessionid.IsRunID,
		},
	}
}

func payloadBytes(payloads [][]byte) int {
	total := 0
	for _, payload := range payloads {
		total += len(payload)
	}
	return total
}

// processCPU is this process's user plus system CPU time so far.
func processCPU(b *testing.B) time.Duration {
	b.Helper()
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		b.Fatalf("getrusage: %v", err)
	}
	return time.Duration(usage.Utime.Nano() + usage.Stime.Nano())
}
