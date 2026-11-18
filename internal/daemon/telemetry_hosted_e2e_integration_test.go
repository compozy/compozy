//go:build integration && !windows

package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/telemetry"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	"github.com/compozy/compozy/internal/testutil/otlptest"
)

// IT-006, IT-007
func TestDaemonE2EOTelNestsHostedToolCallsInTheirTurn(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	t.Run("Should nest each hosted tool call and its execution under the turn of its own session", func(t *testing.T) {
		t.Parallel()

		collector := otlptest.NewCollector(t)
		fixture := filepath.Join(t.TempDir(), "hosted_tool_fixture.json")
		if err := os.WriteFile(fixture, []byte(hostedToolFixture), 0o600); err != nil {
			t.Fatalf("write fixture error = %v", err)
		}
		harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			MockAgents: []e2etest.MockAgentSpec{{
				FixturePath:  fixture,
				FixtureAgent: "hosted-caller",
				AgentName:    "mock-hosted-caller",
			}},
			ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *compozyconfig.Config) {
				cfg.Observability.OTel.Enabled = true
				cfg.Observability.OTel.Endpoint = collector.URL
				cfg.Observability.OTel.Metrics = false
			}},
		})
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
		defer cancel()

		sessionIDs := make([]string, 2)
		for i := range sessionIDs {
			name := fmt.Sprintf("otel-hosted-%d", i)
			sessionIDs[i] = createFixtureBackedSession(t, ctx, harness, "mock-hosted-caller", name).ID
		}
		var wg sync.WaitGroup
		for _, sessionID := range sessionIDs {
			wg.Go(func() {
				if stream, err := harness.PromptSession(
					ctx,
					sessionID,
					"list sessions",
				); err != nil ||
					len(stream) == 0 {
					t.Errorf("PromptSession(%s) = %d updates, %v; want a completed turn", sessionID, len(stream), err)
				}
			})
		}
		wg.Wait()

		for _, sessionID := range sessionIDs {
			spans := waitForHostedToolTrace(ctx, t, collector, sessionID)
			turn := spanNamed(t, spans, telemetry.SpanSessionTurn)
			call := spanNamed(t, spans, telemetry.SpanMCPHostedCall)
			execute := spanNamed(t, spans, telemetry.SpanToolExecute)
			if call.ParentSpanID != turn.SpanID || execute.ParentSpanID != call.SpanID {
				t.Fatalf("%s: hosted call parent %s, tool.execute parent %s; want turn %s → call %s",
					sessionID, call.ParentSpanID, execute.ParentSpanID, turn.SpanID, call.SpanID)
			}
			runID := turn.Attributes[string(telemetry.AttrRunID)]
			if runID == nil || call.Attributes[string(telemetry.AttrRunID)] != runID {
				t.Fatalf("%s: hosted call run id %v, turn run id %v; want the turn's run",
					sessionID, call.Attributes[string(telemetry.AttrRunID)], runID)
			}
			for _, span := range []otlptest.Span{call, execute} {
				if span.Attributes[string(telemetry.AttrTool)] != "compozy__session_list" ||
					span.Attributes[string(telemetry.AttrOutcome)] != telemetry.OutcomeOK {
					t.Fatalf(
						"%s: %s attributes %v, want the native tool id and ok",
						sessionID,
						span.Name,
						span.Attributes,
					)
				}
			}
			if execute.Attributes[string(telemetry.AttrToolRisk)] != "read" {
				t.Fatalf("%s: tool.execute attributes %v, want risk read", sessionID, execute.Attributes)
			}
		}
	})
}

// hostedToolFixture calls a native tool through the hosted MCP proxy the daemon injects into the ACP session.
const hostedToolFixture = `{
	"version": 2,
	"agents": [{
		"name": "hosted-caller",
		"provider": "acpmock",
		"permissions": "approve-all",
		"turns": [{
			"name": "list",
			"match": {"turn_source": "user", "user_text": "list sessions"},
			"steps": [
				{"kind": "native_tool_call", "tool_id": "compozy__session_list", "tool_call_id": "list-1", "raw_input": {}},
				{"kind": "assistant", "text": "listed"}
			]
		}]
	}]
}`

// waitForHostedToolTrace polls the collector until sessionID's turn trace holds its hosted call and tool execution.
func waitForHostedToolTrace(
	ctx context.Context,
	t *testing.T,
	collector *otlptest.Collector,
	sessionID string,
) []otlptest.Span {
	t.Helper()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		spans := collector.Spans()
		for _, span := range spans {
			if span.Name != telemetry.SpanSessionTurn || span.Attributes[string(telemetry.AttrSessionID)] != sessionID {
				continue
			}
			trace := spansInTrace(spans, span.TraceID)
			if hasSpanNamed(trace, telemetry.SpanMCPHostedCall) && hasSpanNamed(trace, telemetry.SpanToolExecute) {
				return trace
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("collector spans = %+v; timed out waiting for the hosted tool trace of %s", spans, sessionID)
		case <-ticker.C:
		}
	}
}
