package acp

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/telemetry"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/otlptest"
	"go.opentelemetry.io/otel/trace"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// Not parallel: every case installs the global OTel providers.
func TestDriverTurnSpans(t *testing.T) {
	t.Run("Should trace provider start-up and the prompt round trip inside the turn", func(t *testing.T) {
		collector := exportACPTraces(t)
		ctx, turn := telemetry.Tracer(telemetry.ScopeSession).Start(testutil.Context(t), telemetry.SpanSessionTurn)
		driver := New()
		proc, err := driver.Start(ctx, StartOpts{
			AgentName:   "helper",
			Command:     helperCommand(t),
			Cwd:         t.TempDir(),
			Env:         helperEnv("echo_prompt_meta", ""),
			Permissions: compozyconfig.PermissionModeApproveAll,
		})
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		defer stopProcess(t, driver, proc)

		events, err := driver.Prompt(ctx, proc, PromptRequest{TurnID: "turn-traced", Message: "hello"})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		echoed := promptMetaEcho(t, collectEvents(t, events))
		turn.End()

		spans := collector.Flush(t)
		spawn := onlySpan(t, spans, telemetry.SpanACPProcessSpawn)
		prompt := onlySpan(t, spans, "acp session/prompt")
		for _, child := range []otlptest.Span{spawn, prompt} {
			if child.ParentSpanID != turn.SpanContext().SpanID() {
				t.Fatalf(
					"%s parent = %s, want session.turn %s",
					child.Name,
					child.ParentSpanID,
					turn.SpanContext().SpanID(),
				)
			}
			if child.Attributes[string(telemetry.AttrOutcome)] != telemetry.OutcomeOK {
				t.Fatalf("%s attributes = %v, want outcome ok", child.Name, child.Attributes)
			}
		}
		if initialize := onlySpan(t, spans, "acp initialize"); initialize.ParentSpanID != spawn.SpanID {
			t.Fatalf("acp initialize parent = %s, want acp.process.spawn %s", initialize.ParentSpanID, spawn.SpanID)
		}
		if prompt.Kind != tracepb.Span_SPAN_KIND_CLIENT ||
			prompt.Attributes[string(telemetry.AttrRPCMethod)] != "session/prompt" ||
			prompt.Attributes[string(telemetry.AttrRPCSystem)] != telemetry.RPCSystemJSONRPC {
			t.Fatalf(
				"acp session/prompt kind %s attributes %v, want a JSON-RPC client span",
				prompt.Kind,
				prompt.Attributes,
			)
		}
		if !prompt.HasEvent(telemetry.EventFirstToken) {
			t.Fatalf("acp session/prompt events = %+v, want first_token", prompt.Events)
		}
		want := fmt.Sprintf("00-%s-%s-01", prompt.TraceID, prompt.SpanID)
		if echoed["traceparent"] != want {
			t.Fatalf("provider _meta = %v, want traceparent %s of the prompt span", echoed, want)
		}
		metrics := collector.FlushMetrics(t)
		if !slices.ContainsFunc(metrics[telemetry.MetricACPRPCDuration].Points, func(point otlptest.Point) bool {
			return point.Count == 1 && point.Attributes[string(telemetry.AttrRPCMethod)] == "session/prompt" &&
				point.Attributes[string(telemetry.AttrOutcome)] == telemetry.OutcomeOK
		}) {
			t.Fatalf("%s = %+v, want one ok session/prompt duration", telemetry.MetricACPRPCDuration,
				metrics[telemetry.MetricACPRPCDuration])
		}
		if ttft := metrics[telemetry.MetricSessionTTFT].Points; len(ttft) != 1 || ttft[0].Count != 1 {
			t.Fatalf("%s points = %+v, want the prompt's first token", telemetry.MetricSessionTTFT, ttft)
		}
	})

	// UT-070, UT-071
	t.Run("Should hand the provider the spawn span's TRACEPARENT only while forwarding", func(t *testing.T) {
		collector := exportACPTraces(t)
		ctx, turn := telemetry.Tracer(telemetry.ScopeSession).Start(testutil.Context(t), telemetry.SpanSessionTurn)
		defer turn.End()
		launchedEnv := func(forward bool) []string {
			launcher := &recordingLauncher{err: errors.New("stop after launch")}
			if _, err := New().Start(ctx, StartOpts{
				AgentName:          "helper",
				Command:            "agent --acp",
				Cwd:                t.TempDir(),
				Env:                []string{"PATH=/usr/bin"},
				Permissions:        compozyconfig.PermissionModeApproveAll,
				Launcher:           launcher,
				ForwardTraceParent: forward,
			}); err == nil {
				t.Fatal("Start() error = nil, want the launcher failure")
			}
			spec, called := launcher.lastSpec()
			if !called {
				t.Fatal("launcher was not called")
			}
			return spec.Env
		}

		forwarded := launchedEnv(true)
		spawn := onlySpan(t, collector.Flush(t), telemetry.SpanACPProcessSpawn)
		want := "TRACEPARENT=" + fmt.Sprintf("00-%s-%s-01", spawn.TraceID, spawn.SpanID)
		if !slices.Contains(forwarded, want) {
			t.Fatalf("forwarding launch env = %v, want %s of acp.process.spawn", forwarded, want)
		}
		for _, entry := range launchedEnv(false) {
			if strings.HasPrefix(entry, "TRACEPARENT=") {
				t.Fatalf("launch env without forwarding has %s", entry)
			}
		}
	})

	// IT-002
	t.Run("Should cover an operator permission wait with an approved span under the turn", func(t *testing.T) {
		collector := exportACPTraces(t)
		ctx, turn := telemetry.Tracer(telemetry.ScopeSession).Start(testutil.Context(t), telemetry.SpanSessionTurn)
		driver := New()
		root := t.TempDir()
		proc := startHelperProcess(t, driver, "permission", filepath.Join(root, "edit.txt"), StartOpts{
			Cwd:         root,
			Permissions: compozyconfig.PermissionModeApproveReads,
		})
		defer stopProcess(t, driver, proc)

		const decisionDelay = 200 * time.Millisecond
		events, err := driver.Prompt(ctx, proc, PromptRequest{TurnID: "turn-permission", Message: "edit"})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		for event := range events {
			if event.Type != EventTypePermission || event.Decision != "" {
				continue
			}
			time.Sleep(decisionDelay)
			if err := driver.ApprovePermission(testutil.Context(t), proc, ApproveRequest{
				RequestID: event.RequestIDValue(), Decision: string(decisionAllowOnce),
			}); err != nil {
				t.Fatalf("ApprovePermission() error = %v", err)
			}
		}
		turn.End()

		wait := onlySpan(t, collector.Flush(t), telemetry.SpanACPPermissionWait)
		if wait.ParentSpanID != turn.SpanContext().SpanID() ||
			wait.Attributes[string(telemetry.AttrOutcome)] != telemetry.OutcomeApproved {
			t.Fatalf("acp.permission.wait parent %s attributes %v, want approved under session.turn %s",
				wait.ParentSpanID, wait.Attributes, turn.SpanContext().SpanID())
		}
		if took := wait.End.Sub(wait.Start); took < decisionDelay || took > decisionDelay+2*time.Second {
			t.Fatalf("acp.permission.wait lasted %s, want about the %s operator delay", took, decisionDelay)
		}
	})
}

// UT-036
func TestActiveTraceContextFollowsTheActivePrompt(t *testing.T) {
	t.Parallel()

	t.Run("Should never pair a turn with another turn's trace context while prompts start and end", func(t *testing.T) {
		t.Parallel()

		proc := newDirectProcess(t, compozyconfig.PermissionModeApproveAll)
		const turns = 200
		stop := make(chan struct{})
		var readers sync.WaitGroup
		mismatches := make(chan string, 1)
		for range 4 {
			readers.Go(func() {
				for {
					select {
					case <-stop:
						return
					default:
					}
					turnID, sc := proc.activeTraceContext()
					if turnID != "" && sc.TraceID() != turnTraceID(turnID) {
						select {
						case mismatches <- fmt.Sprintf("turn %s read trace %s", turnID, sc.TraceID()):
						default:
						}
					}
				}
			})
		}
		for index := range turns {
			turnID := fmt.Sprintf("turn-%d", index)
			sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: turnTraceID(turnID), SpanID: trace.SpanID{1}})
			active, err := proc.beginPromptState(turnID, "", 0, sc, 1)
			if err != nil {
				t.Fatalf("beginPromptState(%s) error = %v", turnID, err)
			}
			proc.endPrompt(active)
		}
		close(stop)
		readers.Wait()
		select {
		case mismatch := <-mismatches:
			t.Fatalf("activeTraceContext() mixed turns: %s", mismatch)
		default:
		}
		if turnID, sc := proc.activeTraceContext(); turnID != "" || sc.IsValid() {
			t.Fatalf("activeTraceContext() after the last prompt = %q %v, want none", turnID, sc)
		}
	})
}

func exportACPTraces(t *testing.T) *otlptest.Collector {
	t.Helper()
	collector := otlptest.NewCollector(t)
	collector.ExportWithMetrics(t, 1, telemetry.Registries{ACPMethods: TracedMethods()})
	return collector
}

func onlySpan(t *testing.T, spans []otlptest.Span, name string) otlptest.Span {
	t.Helper()
	var matched []otlptest.Span
	for _, span := range spans {
		if span.Name == name {
			matched = append(matched, span)
		}
	}
	if len(matched) != 1 {
		t.Fatalf("%s spans = %d, want 1 (exported: %+v)", name, len(matched), spans)
	}
	return matched[0]
}

// promptMetaEcho decodes the ACP _meta the echo_prompt_meta helper sent back as message text.
func promptMetaEcho(t *testing.T, events []AgentEvent) map[string]any {
	t.Helper()
	for _, event := range events {
		if event.Type != EventTypeAgentMessage || !strings.HasPrefix(event.Text, "{") {
			continue
		}
		var meta map[string]any
		if err := json.Unmarshal([]byte(event.Text), &meta); err != nil {
			t.Fatalf("decode echoed _meta %q error = %v", event.Text, err)
		}
		return meta
	}
	t.Fatalf("prompt events = %+v, want the echoed _meta", events)
	return nil
}

// turnTraceID derives a distinct trace ID from a turn ID.
func turnTraceID(turnID string) trace.TraceID {
	var id trace.TraceID
	copy(id[:], fmt.Sprintf("%016s", turnID))
	return id
}
