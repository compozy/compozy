package mcp

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/telemetry"
	"github.com/compozy/compozy/internal/testutil/otlptest"
	"github.com/compozy/compozy/internal/tools"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const hostedSpanTool = "compozy__alpha"

// TestHostedCallSpans installs the global telemetry providers, so it does not run in parallel.
func TestHostedCallSpans(t *testing.T) {
	collector := otlptest.NewCollector(t)
	collector.Export(t, 1, telemetry.Registries{
		ToolIDs: func(string) bool { return true },
		IDs: map[attribute.Key]func(string) bool{
			telemetry.AttrRunID: func(string) bool { return true },
		},
	})

	t.Run("Should parent concurrent calls by the run each tool observes", func(t *testing.T) { // UT-031
		registry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView(hostedSpanTool)}}
		service, peer := newHostedSpanService(t, registry)
		const sessions, cycles = 50, 3
		binds := make([]string, sessions)
		nonces := hostedSpanNonces{}
		for i := range sessions {
			binds[i] = nonces.bind(t, service, peer, fmt.Sprintf("sess-span-%d", i))
		}
		turns := make(map[trace.SpanID]string)
		var turnsMu sync.Mutex
		var wg sync.WaitGroup
		for i := range sessions {
			wg.Go(func() {
				sessionID := fmt.Sprintf("sess-span-%d", i)
				for cycle := range cycles {
					runID := fmt.Sprintf("run-%d-%d", i, cycle)
					ctx, turn := startHostedSpanTurn(t.Context(), runID)
					turnsMu.Lock()
					turns[turn.SpanContext().SpanID()] = runID
					turnsMu.Unlock()
					if err := service.BindRun(ctx, sessionID, runID, int64(cycle+1)); err != nil {
						t.Errorf("BindRun(%s) error = %v", runID, err)
					}
					if _, err := service.Call(t.Context(), hostedSpanCall(binds[i]), peer); err != nil {
						t.Errorf("Call(%s) error = %v", runID, err)
					}
					service.ReleaseRun(sessionID, runID, int64(cycle+1))
					turn.End()
				}
			})
		}
		wg.Wait()

		calls := hostedSpans(collector.Flush(t), telemetry.SpanMCPHostedCall)
		toolRunIDs := make(map[string]int)
		registry.mu.Lock()
		recorded := slices.Clone(registry.calls)
		registry.mu.Unlock()
		for _, call := range recorded {
			if call.ToolID == hostedSpanTool {
				toolRunIDs[call.RunID]++
			}
		}
		matched := 0
		for _, span := range calls {
			runID, ok := turns[span.ParentSpanID]
			if !ok {
				continue
			}
			matched++
			if span.Attributes[string(telemetry.AttrRunID)] != runID || toolRunIDs[runID] != 1 ||
				span.Attributes[string(telemetry.AttrOutcome)] != telemetry.OutcomeOK {
				t.Fatalf(
					"hosted call under %s = %+v, tool run ids %v, want the parent's run id",
					runID,
					span,
					toolRunIDs,
				)
			}
		}
		if matched != sessions*cycles {
			t.Fatalf("hosted calls parented by a turn = %d, want %d", matched, sessions*cycles)
		}
	})

	t.Run("Should root a call after its run ended with a link to that run", func(t *testing.T) { // UT-032
		registry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView(hostedSpanTool)}}
		service, peer := newHostedSpanService(t, registry)
		bindID := hostedSpanNonces{}.bind(t, service, peer, "sess-after-run")
		ctx, turn := startHostedSpanTurn(t.Context(), "run-after")
		if err := service.BindRun(ctx, "sess-after-run", "run-after", 1); err != nil {
			t.Fatalf("BindRun() error = %v", err)
		}
		service.ReleaseRun("sess-after-run", "run-after", 1)
		turn.End()
		if _, err := service.Call(t.Context(), hostedSpanCall(bindID), peer); err != nil {
			t.Fatalf("Call() error = %v", err)
		}

		span := lastHostedSpan(t, collector.Flush(t))
		if span.ParentSpanID.IsValid() || len(span.Links) != 1 ||
			span.Links[0].SpanID() != turn.SpanContext().SpanID() ||
			span.Attributes[string(telemetry.AttrOutcome)] != telemetry.OutcomeAfterRun {
			t.Fatalf("hosted call after the run = %+v, want a root linked to %s", span, turn.SpanContext().SpanID())
		}
		if _, ok := span.Attributes[string(telemetry.AttrRunID)]; ok {
			t.Fatalf("hosted call after the run carries run id %v, want none", span.Attributes)
		}
	})

	t.Run("Should root a call through an unknown bind and keep its error", func(t *testing.T) { // UT-033
		registry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView(hostedSpanTool)}}
		service, peer := newHostedSpanService(t, registry)
		if _, err := service.Call(t.Context(), hostedSpanCall("bind-unknown"), peer); !errors.Is(
			err,
			ErrHostedBindNotFound,
		) {
			t.Fatalf("Call(unknown bind) error = %v, want ErrHostedBindNotFound", err)
		}

		span := lastHostedSpan(t, collector.Flush(t))
		if span.ParentSpanID.IsValid() || len(span.Links) != 0 ||
			span.Attributes[string(telemetry.AttrOutcome)] != telemetry.OutcomeUnbound {
			t.Fatalf("hosted call through an unknown bind = %+v, want an unbound root", span)
		}
	})

	t.Run("Should parent a late call of an ended run by the run bound now", func(t *testing.T) { // UT-034, IT-016
		registry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView(hostedSpanTool)}}
		service, peer := newHostedSpanService(t, registry)
		bindID := hostedSpanNonces{}.bind(t, service, peer, "sess-late")
		ctxA, turnA := startHostedSpanTurn(t.Context(), "run-a")
		if err := service.BindRun(ctxA, "sess-late", "run-a", 1); err != nil {
			t.Fatalf("BindRun(A) error = %v", err)
		}
		service.ReleaseRun("sess-late", "run-a", 1)
		turnA.End()
		ctxB, turnB := startHostedSpanTurn(t.Context(), "run-b")
		defer turnB.End()
		if err := service.BindRun(ctxB, "sess-late", "run-b", 2); err != nil {
			t.Fatalf("BindRun(B) error = %v", err)
		}
		if _, err := service.Call(t.Context(), hostedSpanCall(bindID), peer); err != nil {
			t.Fatalf("Call(issued during A) error = %v", err)
		}

		span := lastHostedSpan(t, collector.Flush(t))
		_, call := registry.lastCall(t)
		if span.ParentSpanID != turnB.SpanContext().SpanID() || call.RunID != "run-b" ||
			span.Attributes[string(telemetry.AttrRunID)] != call.RunID {
			t.Fatalf("late hosted call = %+v with tool run %q, want parent and run id of B", span, call.RunID)
		}
	})

	t.Run(
		"Should keep the run span until a matching release and clear it with the launch",
		func(t *testing.T) { // UT-035
			registry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView(hostedSpanTool)}}
			service, peer := newHostedSpanService(t, registry)
			hostedSpanNonces{}.bind(t, service, peer, "sess-release")
			ctx, turn := startHostedSpanTurn(t.Context(), "run-release")
			defer turn.End()
			if err := service.BindRun(ctx, "sess-release", "run-release", 3); err != nil {
				t.Fatalf("BindRun() error = %v", err)
			}
			runSpan := turn.SpanContext()

			service.ReleaseRun("sess-release", "run-release", 2)
			if got := hostedLaunchTrace(service, "sess-release"); !got.runSpanCtx.Equal(runSpan) ||
				got.lastRunSpanCtx.IsValid() {
				t.Fatalf("run trace after a stale release = %+v, want the current run span only", got)
			}
			service.ReleaseRun("sess-release", "run-release", 3)
			if got := hostedLaunchTrace(service, "sess-release"); got.runSpanCtx.IsValid() ||
				!got.lastRunSpanCtx.Equal(runSpan) {
				t.Fatalf("run trace after the matching release = %+v, want it moved to last", got)
			}
			service.ReleaseSession("sess-release")
			if launch := service.launchRecord("sess-release"); launch != nil {
				t.Fatalf("launch after ReleaseSession = %+v, want it and its run trace deleted", launch)
			}
		},
	)

	// IT-016: the proxy carries no trace state, so the binding transitions the session manager drives are the
	// integration point; the real proxy path is covered by the daemon E2E IT-007.
	t.Run("Should keep a replaced provider process under the turn that rebinds it", func(t *testing.T) {
		registry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView(hostedSpanTool)}}
		service, peer := newHostedSpanService(t, registry)
		hostedSpanNonces{}.bind(t, service, peer, "sess-replace")
		ctx, turn := startHostedSpanTurn(t.Context(), "run-replace")
		defer turn.End()
		if err := service.BindRun(ctx, "sess-replace", "run-replace", 1); err != nil {
			t.Fatalf("BindRun() error = %v", err)
		}
		service.ReleaseSession("sess-replace")
		replacement := hostedSpanNonces{}.bind(t, service, peer, "sess-replace")
		if err := service.BindRun(ctx, "sess-replace", "run-replace", 2); err != nil {
			t.Fatalf("BindRun(replacement process) error = %v", err)
		}
		if _, err := service.Call(t.Context(), hostedSpanCall(replacement), peer); err != nil {
			t.Fatalf("Call(replacement process) error = %v", err)
		}

		span := lastHostedSpan(t, collector.Flush(t))
		_, call := registry.lastCall(t)
		if span.ParentSpanID != turn.SpanContext().SpanID() || call.RunID != "run-replace" ||
			span.Attributes[string(telemetry.AttrRunID)] != call.RunID {
			t.Fatalf("call after process replacement = %+v with tool run %q, want the original turn", span, call.RunID)
		}
	})

	t.Run("Should keep calls through a replacement proxy under the same run", func(t *testing.T) { // UT-045, IT-020
		registry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView(hostedSpanTool)}}
		service, peer := newHostedSpanService(t, registry)
		nonces := hostedSpanNonces{}
		first := nonces.bind(t, service, peer, "sess-proxy")
		ctx, turn := startHostedSpanTurn(t.Context(), "run-proxy")
		defer turn.End()
		if err := service.BindRun(ctx, "sess-proxy", "run-proxy", 4); err != nil {
			t.Fatalf("BindRun() error = %v", err)
		}
		service.ReleaseBind(first)
		replacement := nonces.bind(t, service, peer, "sess-proxy")
		if _, err := service.Call(t.Context(), hostedSpanCall(replacement), peer); err != nil {
			t.Fatalf("Call(replacement proxy) error = %v", err)
		}

		span := lastHostedSpan(t, collector.Flush(t))
		_, call := registry.lastCall(t)
		if span.ParentSpanID != turn.SpanContext().SpanID() || call.RunID != "run-proxy" ||
			span.Attributes[string(telemetry.AttrRunID)] != call.RunID {
			t.Fatalf("call through the replacement proxy = %+v with tool run %q, want the bound run", span, call.RunID)
		}
	})
}

// newHostedSpanService uses random bind ids: the counter of newHostedTestService repeats ids across 50 sessions.
func newHostedSpanService(t *testing.T, registry tools.Registry) (*HostedService, PeerInfo) {
	t.Helper()
	executable := hostedTestExecutable(t, "compozy")
	service, err := NewHostedService(&HostedConfig{
		Enabled:        true,
		BindNonceTTL:   time.Minute,
		ExpectedBinary: executable,
		Registry:       func() tools.Registry { return registry },
	})
	if err != nil {
		t.Fatalf("NewHostedService() error = %v", err)
	}
	return service, hostedTestPeer(executable)
}

// hostedSpanNonces remembers each session's launch nonce, which a replacement proxy binds with again.
type hostedSpanNonces map[string]string

// bind launches and arms the session when it is new, then binds one proxy.
func (n hostedSpanNonces) bind(t *testing.T, service *HostedService, peer PeerInfo, sessionID string) string {
	t.Helper()
	if _, ok := n[sessionID]; !ok {
		launch, err := service.Launch(t.Context(), HostedLaunchRequest{SessionID: sessionID, WorkspaceID: "ws-1"})
		if err != nil {
			t.Fatalf("Launch(%s) error = %v", sessionID, err)
		}
		if err := service.ArmLaunch(t.Context(), sessionID); err != nil {
			t.Fatalf("ArmLaunch(%s) error = %v", sessionID, err)
		}
		n[sessionID] = hostedLaunchNonce(t, launch.Args)
	}
	bind, err := service.Bind(t.Context(), HostedBindRequest{SessionID: sessionID, Nonce: n[sessionID]}, peer)
	if err != nil {
		t.Fatalf("Bind(%s) error = %v", sessionID, err)
	}
	return bind.BindID
}

func startHostedSpanTurn(ctx context.Context, runID string) (context.Context, trace.Span) {
	ctx, span := telemetry.Tracer(telemetry.ScopeSession).Start(ctx, telemetry.SpanSessionTurn)
	span.SetAttributes(telemetry.AttrRunID.String(runID))
	return ctx, span
}

func hostedSpanCall(bindID string) HostedCallRequest {
	return HostedCallRequest{BindID: bindID, ToolName: hostedSpanTool}
}

func hostedLaunchTrace(service *HostedService, sessionID string) hostedRunTrace {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.launches[sessionID].runTrace
}

func hostedSpans(spans []otlptest.Span, name string) []otlptest.Span {
	var matched []otlptest.Span
	for _, span := range spans {
		if span.Name == name {
			matched = append(matched, span)
		}
	}
	return matched
}

func lastHostedSpan(t *testing.T, spans []otlptest.Span) otlptest.Span {
	t.Helper()
	calls := hostedSpans(spans, telemetry.SpanMCPHostedCall)
	if len(calls) == 0 {
		t.Fatalf("no %s span exported (exported: %+v)", telemetry.SpanMCPHostedCall, spans)
	}
	return calls[len(calls)-1]
}
