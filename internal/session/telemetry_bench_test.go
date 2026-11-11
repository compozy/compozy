//go:build !windows

package session

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
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
// call, and a usage event. Enabled mode exports to an in-process OTLP/HTTP receiver and installs the daemon's
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
			if mode.export {
				collector = otlptest.NewCollector(b)
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
				traceBytes, spans = payloadBytes(collector.Payloads(otlptest.TracesPath)), len(collector.Spans())
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
				traces := payloadBytes(collector.Payloads(otlptest.TracesPath)) - traceBytes
				metrics := collector.Payloads(otlptest.MetricsPath)
				if traces == 0 || len(metrics) == 0 {
					b.Fatalf("receiver got %d trace bytes and %d metric pushes, want both", traces, len(metrics))
				}
				otlpBytes = float64(traces)/float64(turns) + float64(len(metrics[len(metrics)-1]))
				turnSpans = float64(len(collector.Spans())-spans) / float64(turns)
			}
			b.ReportMetric(otlpBytes, "otlp_bytes/turn")
			b.ReportMetric(turnSpans, "spans/turn")
		})
	}
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
