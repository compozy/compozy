//go:build integration && !windows

package daemon

import (
	"context"
	"maps"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	automationpkg "github.com/compozy/compozy/internal/automation"
	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/telemetry"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	"github.com/compozy/compozy/internal/testutil/otlptest"
	"go.opentelemetry.io/otel/trace"
)

func TestDaemonE2EOTelTracesBackgroundWork(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	// IT-009
	t.Run("Should root an automation job's prompted turn under automation.run", func(t *testing.T) {
		t.Parallel()

		collector := otlptest.NewCollector(t)
		harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			ConfigSeed: e2etest.ConfigSeedOptions{
				DefaultAgent: automationTaskFixtureAgentName,
				Mutate:       exportTracesTo(collector),
			},
			MockAgents: []e2etest.MockAgentSpec{{
				FixturePath:  mockFixturePath(t, "automation_task_fixture.json"),
				FixtureAgent: "automation-runner",
				AgentName:    automationTaskFixtureAgentName,
			}},
		})
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
		defer cancel()

		seeded, err := harness.SeedAutomationFixtures(ctx, e2etest.AutomationFixtureSeed{
			Jobs: []compozycontract.CreateJobRequest{{
				Scope:       automationpkg.AutomationScopeWorkspace,
				WorkspaceID: harness.WorkspaceID,
				Name:        "otel-deploy-review",
				AgentName:   automationTaskFixtureAgentName,
				Prompt:      "Review payload deploy for main",
				Schedule:    automationpkg.ScheduleSpec{Mode: automationpkg.ScheduleModeEvery, Interval: "1h"},
			}},
		})
		if err != nil || len(seeded.Jobs) != 1 {
			t.Fatalf("SeedAutomationFixtures(job) = %+v, %v; want one job", seeded, err)
		}
		job := seeded.Jobs[0]
		if _, err := harness.TriggerAutomationJob(ctx, job.ID); err != nil {
			t.Fatalf("TriggerAutomationJob(%q) error = %v", job.ID, err)
		}

		spans := waitForRootTrace(ctx, t, collector, telemetry.SpanAutomationRun, func(spans []otlptest.Span) bool {
			return len(spansNamed(spans, telemetry.SpanSessionTurn)) == 1
		})
		root := spanNamed(t, spans, telemetry.SpanAutomationRun)
		if got := root.Attributes[string(telemetry.AttrAutomationID)]; got != job.ID {
			t.Fatalf("automation.run %s = %v, want job id %s", telemetry.AttrAutomationID, got, job.ID)
		}
		turn := spanNamed(t, spans, telemetry.SpanSessionTurn)
		requireDescendant(t, spans, turn, root)
	})

	// IT-010
	t.Run("Should root a two-node Loop's turns and their hook dispatches under loop.run", func(t *testing.T) {
		t.Parallel()

		collector := otlptest.NewCollector(t)
		harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			ConfigSeed: e2etest.ConfigSeedOptions{Mutate: exportTracesTo(collector)},
			MockAgents: []e2etest.MockAgentSpec{{
				FixturePath:  mockFixturePath(t, "loop_events_fixture.json"),
				FixtureAgent: "loop_events",
				AgentName:    "loop-events-agent",
			}},
			Workspace: e2etest.WorkspaceSeedOptions{Files: map[string]string{
				".compozy/config.toml": `[[hooks.declarations]]
name = "otel-turn-start"
event = "turn.start"
mode = "sync"
command = "/bin/echo"
args = ['{}']
`,
			}},
		})
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
		defer cancel()

		createLoopViaHTTP(t, ctx, harness, twoNodeTraceLoopDefinition())
		run := runLoopViaHTTP(t, ctx, harness, "otel-two-node-probe")
		waitForLoopRunStatus(t, ctx, harness, run.ID, compozycontract.LoopRunStatusDone)

		spans := waitForRootTrace(ctx, t, collector, telemetry.SpanLoopRun, func(spans []otlptest.Span) bool {
			return len(spansNamed(spans, telemetry.SpanSessionTurn)) == 2 &&
				len(spansNamed(spans, telemetry.SpanHookDispatch)) >= 2
		})
		root := spanNamed(t, spans, telemetry.SpanLoopRun)
		if got := root.Attributes[string(telemetry.AttrLoopID)]; got != run.ID {
			t.Fatalf("loop.run %s = %v, want run id %s", telemetry.AttrLoopID, got, run.ID)
		}
		turns := spansNamed(spans, telemetry.SpanSessionTurn)
		for _, turn := range turns {
			requireDescendant(t, spans, turn, root)
		}
		for _, turn := range turns {
			if !hasTurnStartDispatch(spans, turn) {
				t.Fatalf("trace spans = %+v, want a turn.start hook.dispatch under session.turn %s", spans, turn.SpanID)
			}
		}
	})
}

func exportTracesTo(collector *otlptest.Collector) func(*compozyconfig.Config) {
	return func(cfg *compozyconfig.Config) {
		cfg.Observability.OTel.Enabled = true
		cfg.Observability.OTel.Endpoint = collector.URL
		cfg.Observability.OTel.Metrics = false
	}
}

// waitForRootTrace polls the collector until a root span named rootName exports and its trace satisfies complete.
func waitForRootTrace(
	ctx context.Context,
	t *testing.T,
	collector *otlptest.Collector,
	rootName string,
	complete func([]otlptest.Span) bool,
) []otlptest.Span {
	t.Helper()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		spans := collector.Spans()
		for _, span := range spans {
			if span.Name != rootName || span.ParentSpanID.IsValid() {
				continue
			}
			if trace := spansInTrace(spans, span.TraceID); complete(trace) {
				return trace
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("collector spans = %+v; timed out waiting for the %s trace", spans, rootName)
		case <-ticker.C:
		}
	}
}

func spansNamed(spans []otlptest.Span, name string) []otlptest.Span {
	var matched []otlptest.Span
	for _, span := range spans {
		if span.Name == name {
			matched = append(matched, span)
		}
	}
	return matched
}

// requireDescendant fails unless span's parent chain within spans reaches ancestor.
func requireDescendant(t *testing.T, spans []otlptest.Span, span otlptest.Span, ancestor otlptest.Span) {
	t.Helper()
	parents := make(map[trace.SpanID]trace.SpanID, len(spans))
	for _, candidate := range spans {
		parents[candidate.SpanID] = candidate.ParentSpanID
	}
	for parent := span.ParentSpanID; parent.IsValid(); parent = parents[parent] {
		if parent == ancestor.SpanID {
			return
		}
	}
	t.Fatalf("trace spans = %+v, want %s %s under %s %s", spans, span.Name, span.SpanID, ancestor.Name, ancestor.SpanID)
}

func hasTurnStartDispatch(spans []otlptest.Span, turn otlptest.Span) bool {
	for _, span := range spansNamed(spans, telemetry.SpanHookDispatch) {
		if span.ParentSpanID == turn.SpanID &&
			span.Attributes[string(telemetry.AttrHookEvent)] == hookspkg.HookTurnStart.String() &&
			span.Attributes[string(telemetry.AttrOutcome)] == telemetry.OutcomeOK {
			return true
		}
	}
	return false
}

// twoNodeTraceLoopDefinition is the Loop events probe with a second independent run-agent node.
func twoNodeTraceLoopDefinition() compozycontract.LoopDefinitionDocument {
	definition := loopEventsDefinition()
	definition.Meta.Name = "otel-two-node-probe"
	definition.Contract.StopWhen = dsl.StopWhenSpec{
		Expr: "nodes.probe.status == 'succeeded' && nodes.second.status == 'succeeded'",
	}
	second := definition.Graph.Nodes[0]
	second.ID = "second"
	second.Params = maps.Clone(second.Params)
	definition.Graph.Nodes = append(definition.Graph.Nodes, second)
	return definition
}
