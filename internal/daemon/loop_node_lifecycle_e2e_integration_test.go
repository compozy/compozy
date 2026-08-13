//go:build integration && !windows

package daemon

import (
	"context"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
)

const (
	lifecycleRetryLoopName      = "node-lifecycle-retry"
	lifecycleRouteLoopName      = "node-lifecycle-route"
	lifecycleEscalationLoopName = "node-lifecycle-escalation"
)

func TestDaemonE2ELoopNodeLifecycleShouldRetryRouteAndEscalate(t *testing.T) {
	acpmock.RequireDriver(t)
	workspaceRoot := t.TempDir()
	seedLoopNodeLifecycleDefinitions(t, workspaceRoot)
	fixturePath := mockFixturePath(t, "loop_node_lifecycle_fixture.json")
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		Workspace: e2etest.WorkspaceSeedOptions{Root: workspaceRoot},
		MockAgents: []e2etest.MockAgentSpec{
			{FixturePath: fixturePath, FixtureAgent: "lifecycle_retry", AgentName: "lifecycle-retry-agent"},
			{FixturePath: fixturePath, FixtureAgent: "lifecycle_route", AgentName: "lifecycle-route-agent"},
			{FixturePath: fixturePath, FixtureAgent: "lifecycle_escalation", AgentName: "lifecycle-escalation-agent"},
		},
		StartTimeout: 30 * time.Second,
	})

	for _, name := range []string{lifecycleRetryLoopName, lifecycleRouteLoopName, lifecycleEscalationLoopName} {
		waitForLoopCatalogEntry(t, t.Context(), harness, name)
	}

	t.Run("Should heal an attempt timeout inside one generation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
		defer cancel()
		run := runLoopViaHTTP(t, ctx, harness, lifecycleRetryLoopName)
		waitForLoopRunStatus(t, ctx, harness, run.ID, compozycontract.LoopRunStatusDone)

		detail := lifecycleRunDetail(t, ctx, harness, run.ID)
		if detail.Run.Generation != 1 || len(detail.Generations) != 1 {
			t.Fatalf("retry generation count = %d/%d, want one", detail.Run.Generation, len(detail.Generations))
		}
		output := lifecycleOutput(t, detail.Generations[0], "primary")
		if output.Status != "succeeded" || !strings.Contains(output.OutputRef, "recovered") {
			t.Fatalf("retry output = %#v, want recovered success", output)
		}
		events := readLoopRunSSEUntil(
			t,
			ctx,
			harness,
			loopRunEventsPath(harness.WorkspaceID, run.ID, 0),
			func(events []loopRunSSEEvent) bool {
				return loopSSEKinds(events).Contains(
					string(compozycontract.LoopRunEventNodeRetryScheduled),
					string(compozycontract.LoopRunEventNodeSucceeded),
				)
			},
		)
		if got := feedbackNodeEventCount(t, events, compozycontract.LoopRunEventNodeRunning, "primary"); got != 2 {
			t.Fatalf("primary node_running events = %d, want two attempts", got)
		}
		lifecycleAssertPromptContains(t, harness, "lifecycle-retry-agent", []string{
			"Automatic retry context:", `"code":"action_timeout"`,
		})
	})

	t.Run("Should activate a fallback and skip the success-only path", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
		defer cancel()
		run := runLoopViaHTTP(t, ctx, harness, lifecycleRouteLoopName)
		waitForLoopRunStatus(t, ctx, harness, run.ID, compozycontract.LoopRunStatusDone)

		detail := lifecycleRunDetail(t, ctx, harness, run.ID)
		generation := detail.Generations[0]
		primary := lifecycleOutput(t, generation, "primary")
		fallback := lifecycleOutput(t, generation, "fallback")
		successOnly := lifecycleOutput(t, generation, "success_only")
		if primary.Status != "succeeded" || primary.OutputRef != "error_routed:fallback" {
			t.Fatalf("routed primary output = %#v", primary)
		}
		if fallback.Status != "succeeded" || !strings.Contains(fallback.OutputRef, "fallback completed") {
			t.Fatalf("fallback output = %#v, want success", fallback)
		}
		if successOnly.TaskRunID != "" || successOnly.OutputRef != "branch_skipped" {
			t.Fatalf("success-only output = %#v, want skipped without a task run", successOnly)
		}
		events := readLoopRunSSEUntil(
			t,
			ctx,
			harness,
			loopRunEventsPath(harness.WorkspaceID, run.ID, 0),
			func(events []loopRunSSEEvent) bool {
				return loopSSEKinds(events).Contains("custom_event", "effect_results")
			},
		)
		assertLifecycleEffectEventContains(t, events, "custom_event", []string{
			`"authored_kind":"lifecycle_failure_notification"`,
			`"cause":"route fallback"`,
			`"node_id":"primary"`,
		})
		assertLifecycleEffectEventContains(t, events, "effect_results", []string{
			`"trigger":"on_error"`, `"outcome":"ok"`,
		})
	})

	t.Run("Should carry an unhandled failure into generation repair", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
		defer cancel()
		run := runLoopViaHTTP(t, ctx, harness, lifecycleEscalationLoopName)
		waitForLoopRunStatus(t, ctx, harness, run.ID, compozycontract.LoopRunStatusDone)

		detail := lifecycleRunDetail(t, ctx, harness, run.ID)
		if detail.Run.Generation != 2 || len(detail.Generations) != 2 {
			t.Fatalf("escalation generation count = %d/%d, want two", detail.Run.Generation, len(detail.Generations))
		}
		if got := lifecycleOutput(t, detail.Generations[0], "primary").Status; got != "failed" {
			t.Fatalf("generation one primary status = %q, want failed", got)
		}
		if output := lifecycleOutput(t, detail.Generations[1], "primary"); output.Status != "succeeded" ||
			!strings.Contains(output.OutputRef, "repair completed") {
			t.Fatalf("generation two primary output = %#v, want repaired success", output)
		}
		lifecycleAssertPromptContains(t, harness, "lifecycle-escalation-agent", []string{
			"escalation generation 2", "semantic escalation", `"disposition":"escalated"`,
		})
	})
}

func seedLoopNodeLifecycleDefinitions(t testing.TB, workspaceRoot string) {
	t.Helper()
	root := filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.LoopsDirName)
	for name, source := range map[string]string{
		lifecycleRetryLoopName:      lifecycleRetryLoopYAML(),
		lifecycleRouteLoopName:      lifecycleRouteLoopYAML(),
		lifecycleEscalationLoopName: lifecycleEscalationLoopYAML(),
	} {
		if _, _, err := looppkg.WriteDefinition(root, []byte(source), looppkg.WriteDefinitionOptions{
			Source: looppkg.SourceWorkspace,
		}); err != nil {
			t.Fatalf("WriteDefinition(%s) error = %v", name, err)
		}
	}
}

func lifecycleRetryLoopYAML() string {
	return `apiVersion: compozy.loop/v1
kind: Loop
meta: { name: node-lifecycle-retry, description: "E2E retry lifecycle probe." }
concurrency: allow
contract:
  goal: "Recover one transient node failure."
  definition_of_done: "The retried node succeeds."
  stop_when: "nodes.primary.status == 'succeeded'"
  verification: []
  terminal_states: [done, failed, blocked, exhausted, stalled]
  iteration_cap: 1
  no_progress: { window: 2, hash_fields: ["nodes.primary.output"] }
  budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: halt }
graph:
  nodes:
    - id: primary
      class: action
      kind: run-agent
      timeout: 2s
      retry: { max_attempts: 2, backoff: { base: 10ms, max: 10ms } }
      params:
        agent: lifecycle-retry-agent
        prompt: "retry lifecycle"
        output_schema:
          type: object
          required: [summary, value]
          properties:
            summary: { type: string }
            value: { type: string }
  edges: []
start: [{ kind: http }, { kind: uds }]
`
}

func lifecycleRouteLoopYAML() string {
	return `apiVersion: compozy.loop/v1
kind: Loop
meta: { name: node-lifecycle-route, description: "E2E error route probe." }
concurrency: allow
contract:
  goal: "Finish through the authored fallback."
  definition_of_done: "The fallback succeeds and the success-only path stays skipped."
  stop_when: "nodes.fallback.status == 'succeeded'"
  verification: []
  terminal_states: [done, failed, blocked, exhausted, stalled]
  iteration_cap: 1
  no_progress: { window: 2, hash_fields: ["nodes.fallback.output"] }
  budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: halt }
graph:
  nodes:
    - id: primary
      class: action
      kind: run-agent
      retry: { max_attempts: 0 }
      result_contract: { failure_field: error, message_field: error }
      on_error:
        route: fallback
        effects:
          - emit:
              kind: lifecycle_failure_notification
              payload:
                cause: "{{ .effect.failure.cause }}"
                node_id: "{{ .effect.identity.node_id }}"
                run_link: "{{ .effect.links.run }}"
      params:
        agent: lifecycle-route-agent
        prompt: "route primary"
        output_schema:
          type: object
          required: [summary]
          properties:
            summary: { type: string }
            error: { type: string }
    - id: fallback
      class: action
      kind: run-agent
      params:
        agent: lifecycle-route-agent
        prompt: "route fallback"
        output_schema:
          type: object
          required: [summary, value]
          properties:
            summary: { type: string }
            value: { type: string }
    - id: success_only
      class: action
      kind: run-agent
      params: { agent: lifecycle-route-agent, prompt: "must never run" }
  edges:
    - { from: primary, to: fallback }
    - { from: primary, to: success_only }
start: [{ kind: http }, { kind: uds }]
`
}

func assertLifecycleEffectEventContains(
	t *testing.T,
	events []loopRunSSEEvent,
	kind string,
	fragments []string,
) {
	t.Helper()
	for _, event := range events {
		if string(event.Kind) != kind {
			continue
		}
		payload := string(event.Payload)
		allFound := true
		for _, fragment := range fragments {
			if !strings.Contains(payload, fragment) {
				allFound = false
				break
			}
		}
		if allFound {
			return
		}
	}
	t.Fatalf("events = %#v, want %s payload containing %v", events, kind, fragments)
}

func lifecycleEscalationLoopYAML() string {
	return `apiVersion: compozy.loop/v1
kind: Loop
meta: { name: node-lifecycle-escalation, description: "E2E escalation probe." }
concurrency: allow
contract:
  goal: "Repair one unhandled node failure."
  definition_of_done: "The next generation succeeds with classified repair context."
  stop_when: "nodes.primary.status == 'succeeded'"
  verification: []
  terminal_states: [done, failed, blocked, exhausted, stalled]
  iteration_cap: 2
  no_progress: { window: 2, hash_fields: ["nodes.primary.output"] }
  budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: halt }
graph:
  nodes:
    - id: primary
      class: action
      kind: run-agent
      retry: { max_attempts: 0 }
      result_contract: { failure_field: error, message_field: error }
      params:
        agent: lifecycle-escalation-agent
        prompt: "escalation generation {{ .generation }}"
        output_schema:
          type: object
          required: [summary]
          properties:
            summary: { type: string }
            value: { type: string }
            error: { type: string }
  edges: []
start: [{ kind: http }, { kind: uds }]
`
}

func lifecycleRunDetail(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	runID string,
) compozycontract.LoopRunResponse {
	t.Helper()
	var detail compozycontract.LoopRunResponse
	path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) +
		"/loop-runs/" + url.PathEscape(runID)
	if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &detail); err != nil {
		t.Fatalf("HTTP loop status error = %v", err)
	}
	return detail
}

func lifecycleOutput(
	t testing.TB,
	generation compozycontract.LoopGenerationPayload,
	nodeID string,
) compozycontract.LoopGenerationOutput {
	t.Helper()
	for _, output := range generation.Outputs {
		if output.NodeID == nodeID {
			return output
		}
	}
	t.Fatalf("generation %d outputs = %#v, want node %s", generation.Generation, generation.Outputs, nodeID)
	return compozycontract.LoopGenerationOutput{}
}

func lifecycleAssertPromptContains(
	t testing.TB,
	harness *e2etest.RuntimeHarness,
	agentName string,
	fragments []string,
) {
	t.Helper()
	registration, ok := harness.MockAgentRegistration(agentName)
	if !ok {
		t.Fatalf("MockAgentRegistration(%q) missing", agentName)
	}
	records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(%q) error = %v", registration.DiagnosticsPath, err)
	}
	for _, record := range acpmock.PromptDiagnostics(records) {
		matched := true
		for _, fragment := range fragments {
			if !strings.Contains(record.Prompt, fragment) {
				matched = false
				break
			}
		}
		if matched {
			return
		}
	}
	t.Fatalf("agent %s prompts do not contain %#v: %#v", agentName, fragments, acpmock.PromptDiagnostics(records))
}
