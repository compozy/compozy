//go:build integration && !windows

package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/telemetry"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	"github.com/compozy/compozy/internal/testutil/otlptest"
	"go.opentelemetry.io/otel/trace"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

const promptRouteSpan = "POST /api/workspaces/:workspace_id/sessions/:session_id/prompt"

// IT-001
func TestDaemonE2EOTelExportsOneTracePerTurn(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	t.Run("Should nest provider start-up and the ACP prompt in the turn under the prompt request", func(t *testing.T) {
		t.Parallel()

		collector := otlptest.NewCollector(t)
		harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			MockAgents: []e2etest.MockAgentSpec{{
				FixturePath:  mockFixturePath(t, "multi_agent_fixture.json"),
				FixtureAgent: "alpha",
				AgentName:    "mock-alpha",
			}},
			ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *compozyconfig.Config) {
				cfg.Observability.OTel.Enabled = true
				cfg.Observability.OTel.Endpoint = collector.URL
				cfg.Observability.OTel.Metrics = false
			}},
		})
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
		defer cancel()

		session := createFixtureBackedSession(t, ctx, harness, "mock-alpha", "otel-turn-session")
		if stream, err := harness.PromptSession(ctx, session.ID, "hello alpha"); err != nil || len(stream) == 0 {
			t.Fatalf("PromptSession() = %d updates, %v; want a completed turn", len(stream), err)
		}

		spans := waitForTurnTrace(ctx, t, collector, session.ID)
		turn := spanNamed(t, spans, telemetry.SpanSessionTurn)
		request := spanNamed(t, spans, promptRouteSpan)
		if turn.ParentSpanID != request.SpanID || request.ParentSpanID.IsValid() {
			t.Fatalf("session.turn parent %s, request parent %s; want the turn under the root prompt request %s",
				turn.ParentSpanID, request.ParentSpanID, request.SpanID)
		}
		spawn := spanNamed(t, spans, telemetry.SpanACPProcessSpawn)
		prompt := spanNamed(t, spans, "acp session/prompt")
		for _, child := range []otlptest.Span{spawn, prompt} {
			if child.ParentSpanID != turn.SpanID {
				t.Fatalf("%s parent = %s, want session.turn %s", child.Name, child.ParentSpanID, turn.SpanID)
			}
		}
		if !prompt.HasEvent(telemetry.EventFirstToken) {
			t.Fatalf("acp session/prompt events = %+v, want first_token", prompt.Events)
		}
		if turn.Start.After(spawn.Start) {
			t.Fatalf("session.turn started at %s, after provider start-up at %s", turn.Start, spawn.Start)
		}
		for _, span := range spans {
			if span.SpanID != turn.SpanID && span.SpanID != request.SpanID && span.End.After(turn.End) {
				t.Fatalf("%s ended at %s, after session.turn ended at %s", span.Name, span.End, turn.End)
			}
		}
		for key, want := range map[string]any{
			string(telemetry.AttrSessionID):   session.ID,
			string(telemetry.AttrWorkspaceID): harness.WorkspaceID,
			string(telemetry.AttrAgent):       "mock-alpha",
			string(telemetry.AttrGenAISystem): acpmock.ProviderName,
			string(telemetry.AttrOutcome):     telemetry.OutcomeOK,
		} {
			if got := turn.Attributes[key]; got != want {
				t.Fatalf("session.turn %s = %v, want %v (attributes %v)", key, got, want, turn.Attributes)
			}
		}
		for _, key := range []string{string(telemetry.AttrTurnID), string(telemetry.AttrRunID)} {
			if turn.Attributes[key] == nil {
				t.Fatalf("session.turn lacks %s (attributes %v)", key, turn.Attributes)
			}
		}
	})
}

// IT-003
func TestDaemonE2EOTelMarksProviderFailedTurns(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	t.Run("Should mark a provider-failed turn with provider_error and no description", func(t *testing.T) {
		t.Parallel()

		collector := otlptest.NewCollector(t)
		fixture := filepath.Join(t.TempDir(), "failing_fixture.json")
		if err := os.WriteFile(fixture, []byte(failingPromptFixture), 0o600); err != nil {
			t.Fatalf("write fixture error = %v", err)
		}
		harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			MockAgents: []e2etest.MockAgentSpec{{
				FixturePath:  fixture,
				FixtureAgent: "failing",
				AgentName:    "mock-failing",
			}},
			ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *compozyconfig.Config) {
				cfg.Observability.OTel.Enabled = true
				cfg.Observability.OTel.Endpoint = collector.URL
				cfg.Observability.OTel.Metrics = false
			}},
		})
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
		defer cancel()

		session := createFixtureBackedSession(t, ctx, harness, "mock-failing", "otel-failing-session")
		if _, err := harness.PromptSession(ctx, session.ID, "fail this turn"); err != nil {
			t.Fatalf("PromptSession() error = %v", err)
		}

		spans := waitForTurnTrace(ctx, t, collector, session.ID)
		for _, name := range []string{telemetry.SpanSessionTurn, "acp session/prompt"} {
			span := spanNamed(t, spans, name)
			if span.StatusCode != tracepb.Status_STATUS_CODE_ERROR || span.StatusMessage != "" ||
				span.Attributes[string(telemetry.AttrErrorType)] != telemetry.ErrorProvider ||
				span.Attributes[string(telemetry.AttrOutcome)] != telemetry.OutcomeError {
				t.Fatalf("%s status %s %q attributes %v, want an error with provider_error and no description",
					name, span.StatusCode, span.StatusMessage, span.Attributes)
			}
		}
	})
}

// failingPromptFixture answers the matched prompt with a JSON-RPC error that is not a recovery candidate.
const failingPromptFixture = `{
	"version": 2,
	"agents": [{
		"name": "failing",
		"provider": "acpmock",
		"permissions": "approve-all",
		"turns": [{
			"name": "fail",
			"match": {"turn_source": "user", "user_text": "fail this turn"},
			"steps": [{
				"kind": "driver_control",
				"driver_control": {"action": "fail_prompt", "error_message": "model backend crashed"}
			}]
		}]
	}]
}`

// waitForTurnTrace polls the collector until the trace holding sessionID's turn has its request span; the daemon
// exports on its batch schedule.
func waitForTurnTrace(
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
			if hasSpanNamed(trace, promptRouteSpan) {
				return trace
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("collector spans = %+v; timed out waiting for the turn trace of %s", spans, sessionID)
		case <-ticker.C:
		}
	}
}

func spansInTrace(spans []otlptest.Span, traceID trace.TraceID) []otlptest.Span {
	var matched []otlptest.Span
	for _, span := range spans {
		if span.TraceID == traceID {
			matched = append(matched, span)
		}
	}
	return matched
}

func hasSpanNamed(spans []otlptest.Span, name string) bool {
	for _, span := range spans {
		if span.Name == name {
			return true
		}
	}
	return false
}

func spanNamed(t *testing.T, spans []otlptest.Span, name string) otlptest.Span {
	t.Helper()
	for _, span := range spans {
		if span.Name == name {
			return span
		}
	}
	t.Fatalf("trace spans = %+v, want %s", spans, name)
	return otlptest.Span{}
}
