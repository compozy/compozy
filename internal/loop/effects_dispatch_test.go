package loop

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

// Invariant: effect identities and rendered payloads are deterministic, pre-bound, sanitized,
// and isolated before the store transaction sees them. This is the canonical pure-effects suite;
// no earlier suite owns the new rendering boundary.
func TestEffectDispatchRendering(t *testing.T) {
	t.Parallel()

	t.Run("Should derive one stable delivery identity per source trigger entry", func(t *testing.T) {
		t.Parallel()

		first := EffectDeliveryID("run-1", "event-1", EffectTriggerOnError, 0)
		replayed := EffectDeliveryID("run-1", "event-1", EffectTriggerOnError, 0)
		sibling := EffectDeliveryID("run-1", "event-1", EffectTriggerOnError, 1)
		if len(first) != 64 || first != replayed || first == sibling {
			t.Fatalf("delivery ids = %q, %q, %q, want stable distinct SHA-256 identities", first, replayed, sibling)
		}
	})

	t.Run("Should pre-bind a paused node identity and decision links before rendering", func(t *testing.T) {
		t.Parallel()

		run := effectTestRun()
		nextAttemptAt := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
		failure := &ClassifiedFailure{
			Class: FailureTransport, Code: "tool_backend_failed", Cause: "upstream unavailable",
			Hint: "retry after checking the endpoint", RetryEligible: true,
		}
		intents, err := RenderEffectIntents([]dsl.EffectSpec{{
			Tool: "known__notify",
			With: map[string]any{
				"body":          "{{ .effect.identity.node_id }}: {{ .effect.failure.cause }}",
				"attempt":       "{{ .effect.attempt.number }}",
				"item_index":    "{{ .effect.identity.item_index }}",
				"run_link":      "{{ .effect.links.run }}",
				"approval_link": "{{ .effect.links.approval }}",
			},
		}}, EffectContextRequest{
			Run: &run, Trigger: EffectTriggerOnPause, Generation: 3,
			NodeID: "fetch", ItemIndex: 0, Attempt: 4, NextAttemptAt: &nextAttemptAt,
			Disposition: AttemptRetried, Failure: failure,
		})
		if err != nil {
			t.Fatalf("RenderEffectIntents() error = %v", err)
		}
		entry := decodeRenderedEffectForTest(t, intents[0].Entry)
		if entry.With["body"] != "fetch: upstream unavailable" || entry.With["attempt"] != float64(4) ||
			entry.With["item_index"] != float64(0) {
			t.Fatalf("rendered with = %#v, want pre-bound failure and attempt", entry.With)
		}
		if entry.With["run_link"] != "/api/workspaces/ws-effect/loop-runs/run-effect" ||
			!strings.Contains(entry.With["approval_link"].(string), "action=approval") ||
			!strings.Contains(entry.With["approval_link"].(string), "generation=3") ||
			!strings.Contains(entry.With["approval_link"].(string), "item_index=0") ||
			!strings.Contains(entry.With["approval_link"].(string), "node_id=fetch") {
			t.Fatalf("rendered links = %#v, want exact run and node decision links", entry.With)
		}
	})

	t.Run("Should redact planted secrets from the persisted rendered input", func(t *testing.T) {
		t.Parallel()

		run := effectTestRun()
		failure := &ClassifiedFailure{
			Class: FailurePayloadDeclared, Code: "remote_error",
			Cause: "request used compozy_claim_SECRET123", RetryEligible: false,
		}
		intents, err := RenderEffectIntents([]dsl.EffectSpec{{
			Tool: "known__notify",
			With: map[string]any{"body": "{{ .effect.failure.cause }}"},
		}}, EffectContextRequest{
			Run: &run, Trigger: EffectTriggerOnError, Generation: 1,
			NodeID: "fetch", Attempt: 1, Disposition: AttemptEscalated, Failure: failure,
		})
		if err != nil {
			t.Fatalf("RenderEffectIntents() error = %v", err)
		}
		if strings.Contains(string(intents[0].Entry), "compozy_claim_SECRET123") {
			t.Fatalf("rendered entry = %s, want planted secret redacted", intents[0].Entry)
		}
	})

	t.Run("Should persist one render failure as data without blocking its sibling", func(t *testing.T) {
		t.Parallel()

		run := effectTestRun()
		intents, err := RenderEffectIntents([]dsl.EffectSpec{
			{Tool: "known__notify", With: map[string]any{"body": "{{ .effect.missing.value }}"}},
			{Emit: &dsl.EmitSpec{Kind: "fetch_failed", Payload: map[string]any{"safe": true}}},
		}, EffectContextRequest{
			Run: &run, Trigger: EffectTriggerOnError, Generation: 1,
			NodeID: "fetch", Attempt: 1, Disposition: AttemptEscalated,
			Failure: &ClassifiedFailure{Class: FailureTransport, Code: "network", Cause: "down"},
		})
		if err != nil {
			t.Fatalf("RenderEffectIntents() error = %v", err)
		}
		if len(intents) != 2 {
			t.Fatalf("intents len = %d, want 2 isolated entries", len(intents))
		}
		first := decodeRenderedEffectForTest(t, intents[0].Entry)
		second := decodeRenderedEffectForTest(t, intents[1].Entry)
		if !first.RenderError || first.Kind != "render_error" || second.Kind != "emit" {
			t.Fatalf("entries = %#v, %#v, want error-as-data then deliverable sibling", first, second)
		}
	})
}

func TestCoordinatorEffectsShouldAttachNodeAndTerminalTriggers(t *testing.T) {
	t.Parallel()

	failureClass := FailureTransport
	timeoutClass := FailureAttemptTimeout
	nextAttemptAt := time.Date(2026, 8, 2, 12, 1, 0, 0, time.UTC)

	t.Run("Should attach node and terminal effects", func(t *testing.T) {
		t.Parallel()

		definition := dsl.Definition{
			Contract: dsl.Contract{ContractLifecycleState: &dsl.ContractLifecycleState{
				TerminalEffects: dsl.TerminalEffects{OnFailed: []dsl.EffectSpec{{
					Emit: &dsl.EmitSpec{Kind: "loop_failed", Payload: map[string]any{
						"scope": "{{ .effect.identity.scope }}",
					}},
				}}},
			}},
			Graph: dsl.Graph{Nodes: []dsl.Node{{
				ID: "fetch", Class: dsl.NodeClassAction, Kind: "known__fetch",
				NodeLifecycleState: &dsl.NodeLifecycleState{
					TriggerEffects: dsl.TriggerEffects{
						OnRetry:   []dsl.EffectSpec{{Emit: &dsl.EmitSpec{Kind: "fetch_retrying"}}},
						OnTimeout: []dsl.EffectSpec{{Emit: &dsl.EmitSpec{Kind: "fetch_timed_out"}}},
					},
					OnError: &dsl.ErrorPolicy{AllowFail: true, Effects: []dsl.EffectSpec{{
						Tool: "known__notify", With: map[string]any{
							"disposition": "{{ .effect.attempt.disposition }}",
							"scope":       "{{ .effect.identity.scope }}",
						},
					}}},
				},
			}}},
		}
		plan := task.CoordinatorCompletionPlan{
			Snapshot: task.GenerationSnapshot{LoopRunID: "run-effect", Generation: 2, Payload: GenerationSnapshotPayload{
				Attempts: []NodeAttempt{
					{LoopRunID: "run-effect", Generation: 2, NodeID: "fetch", Attempt: 1,
						Disposition: AttemptRetried, FailureClass: &failureClass, FailureCode: "network",
						Cause: "down", NextAttemptAt: &nextAttemptAt, StartedAt: nextAttemptAt.Add(-time.Second)},
					{LoopRunID: "run-effect", Generation: 2, NodeID: "fetch", Attempt: 2,
						Disposition: AttemptAbsorbed, FailureClass: &timeoutClass, FailureCode: "deadline",
						Cause: "still down", StartedAt: nextAttemptAt},
				},
				Events: []GenerationLifecycleEventIntent{{
					Kind: GenerationLifecycleEventNodeRetryScheduled, NodeID: "fetch", Attempt: 2,
					IssuedEpoch: 2, NextAttemptAt: &nextAttemptAt, FailureClass: failureClass,
				}},
			}},
			Terminal: &task.CoordinatorTerminal{Status: string(StatusFailed)},
		}
		if err := attachCoordinatorEffectIntents(
			effectTestRun(),
			&ResolvedDefinition{Definition: definition},
			&plan,
		); err != nil {
			t.Fatalf("attachCoordinatorEffectIntents() error = %v", err)
		}
		payload, err := GenerationSnapshotPayloadFrom(plan.Snapshot.Payload)
		if err != nil {
			t.Fatalf("GenerationSnapshotPayloadFrom() error = %v", err)
		}
		if len(payload.Events) != 2 || len(payload.Events[0].Effects) != 1 ||
			payload.Events[1].Disposition != AttemptAbsorbed || len(payload.Events[1].Effects) != 2 ||
			payload.Events[1].Effects[1].Trigger != EffectTriggerOnTimeout ||
			len(payload.BoundaryEffects[StatusFailed]) != 1 {
			t.Fatalf("effect-bearing payload = %#v, want retry, absorbed on_error, and terminal effects", payload)
		}
		entry := decodeRenderedEffectForTest(t, payload.Events[1].Effects[0].Entry)
		if entry.With["disposition"] != string(AttemptAbsorbed) || entry.With["scope"] != "node" {
			t.Fatalf("on_error with = %#v, want absorbed disposition in node scope", entry.With)
		}
		terminal := decodeRenderedEffectForTest(t, payload.BoundaryEffects[StatusFailed][0].Entry)
		if terminal.Emit == nil || terminal.Emit.Payload["scope"] != "loop" {
			t.Fatalf("terminal effect = %#v, want distinct loop scope", terminal)
		}
	})

	t.Run("Should attach quarantine context", func(t *testing.T) {
		t.Parallel()

		quarantineEntry := json.RawMessage(`{
		"node_id":"fetch",
		"input_ref":"loop-run:run-effect:node:fetch:input",
		"episodes":[{"generation":2,"quarantined_at":"2026-08-02T12:01:00Z","attempts":[]}]
	}`)
		quarantinePlan := task.CoordinatorCompletionPlan{
			Snapshot: task.GenerationSnapshot{LoopRunID: "run-effect", Generation: 2, Payload: GenerationSnapshotPayload{
				Attempts: []NodeAttempt{{
					LoopRunID: "run-effect", Generation: 2, NodeID: "fetch", Attempt: 3,
					Disposition: AttemptQuarantined, FailureClass: &failureClass,
					FailureCode: "network", Cause: "down", StartedAt: nextAttemptAt,
				}},
				Events: []GenerationLifecycleEventIntent{{
					Kind: GenerationLifecycleEventNodeQuarantined, NodeID: "fetch", Attempt: 3,
					Failure:     &ClassifiedFailure{Class: FailureTransport, Code: "network", Cause: "down"},
					Disposition: AttemptQuarantined, QuarantineEntry: quarantineEntry,
				}},
			}},
		}
		quarantineDefinition := dsl.Definition{Graph: dsl.Graph{Nodes: []dsl.Node{{
			ID: "fetch", Class: dsl.NodeClassAction, Kind: "known__fetch",
			NodeLifecycleState: &dsl.NodeLifecycleState{TriggerEffects: dsl.TriggerEffects{
				OnQuarantine: []dsl.EffectSpec{{Emit: &dsl.EmitSpec{
					Kind:    "fetch_quarantined",
					Payload: map[string]any{"node_id": "{{ .effect.quarantine.node_id }}"},
				}}},
			}},
		}}}}
		if err := attachCoordinatorEffectIntents(
			effectTestRun(),
			&ResolvedDefinition{Definition: quarantineDefinition},
			&quarantinePlan,
		); err != nil {
			t.Fatalf("attachCoordinatorEffectIntents(quarantine) error = %v", err)
		}
		quarantinePayload, err := GenerationSnapshotPayloadFrom(quarantinePlan.Snapshot.Payload)
		if err != nil {
			t.Fatalf("GenerationSnapshotPayloadFrom(quarantine) error = %v", err)
		}
		if len(quarantinePayload.Events) != 1 || len(quarantinePayload.Events[0].Effects) != 1 ||
			quarantinePayload.Events[0].Effects[0].Trigger != EffectTriggerOnQuarantine {
			t.Fatalf("quarantine event = %#v, want one on_quarantine effect", quarantinePayload.Events)
		}
		quarantineEffect := decodeRenderedEffectForTest(t, quarantinePayload.Events[0].Effects[0].Entry)
		if quarantineEffect.Emit == nil || quarantineEffect.Emit.Payload["node_id"] != "fetch" {
			t.Fatalf("quarantine effect = %#v, want entry context", quarantineEffect)
		}
	})

	t.Run("Should treat an undeclared quarantine effect as a no-op", func(t *testing.T) {
		t.Parallel()

		plan := task.CoordinatorCompletionPlan{
			Snapshot: task.GenerationSnapshot{LoopRunID: "run-effect", Generation: 2, Payload: GenerationSnapshotPayload{
				Attempts: []NodeAttempt{{
					LoopRunID: "run-effect", Generation: 2, NodeID: "fetch", Attempt: 1,
					Disposition: AttemptQuarantined, FailureClass: &failureClass, StartedAt: nextAttemptAt,
				}},
			}},
		}
		definition := dsl.Definition{Graph: dsl.Graph{Nodes: []dsl.Node{{
			ID: "fetch", Class: dsl.NodeClassAction, Kind: "known__fetch",
			NodeLifecycleState: &dsl.NodeLifecycleState{},
		}}}}
		if err := attachCoordinatorEffectIntents(
			effectTestRun(), &ResolvedDefinition{Definition: definition}, &plan,
		); err != nil {
			t.Fatalf("attachCoordinatorEffectIntents(no quarantine effects) error = %v", err)
		}
	})
}

func TestCoordinatorEffectsShouldAttachEveryDeclaredTerminalOutcome(t *testing.T) {
	t.Parallel()

	emit := func(kind string) []dsl.EffectSpec {
		return []dsl.EffectSpec{{Emit: &dsl.EmitSpec{Kind: kind}}}
	}
	definition := dsl.Definition{Contract: dsl.Contract{ContractLifecycleState: &dsl.ContractLifecycleState{
		TerminalEffects: dsl.TerminalEffects{
			OnDone: emit("done"), OnNoOp: emit("noop"), OnBlocked: emit("blocked"),
			OnFailed: emit("failed"), OnExhausted: emit("exhausted"), OnStalled: emit("stalled"),
			OnCanceled: emit("canceled"),
		},
	}}}
	cases := []struct {
		status  Status
		trigger EffectTrigger
	}{
		{StatusDone, EffectTriggerOnDone},
		{StatusNoOp, EffectTriggerOnNoOp},
		{StatusBlocked, EffectTriggerOnBlocked},
		{StatusFailed, EffectTriggerOnFailed},
		{StatusExhausted, EffectTriggerOnExhausted},
		{StatusStalled, EffectTriggerOnStalled},
		{StatusCanceled, EffectTriggerOnCanceled},
	}
	for _, testCase := range cases {
		t.Run("Should attach "+string(testCase.status)+" to its exact committed outcome", func(t *testing.T) {
			t.Parallel()

			plan := task.CoordinatorCompletionPlan{
				Snapshot: task.GenerationSnapshot{LoopRunID: "run-effect", Generation: 2},
				Terminal: &task.CoordinatorTerminal{Status: string(testCase.status)},
			}
			if err := attachCoordinatorEffectIntents(
				effectTestRun(),
				&ResolvedDefinition{Definition: definition},
				&plan,
			); err != nil {
				t.Fatalf("attachCoordinatorEffectIntents() error = %v", err)
			}
			payload, err := GenerationSnapshotPayloadFrom(plan.Snapshot.Payload)
			if err != nil {
				t.Fatalf("GenerationSnapshotPayloadFrom() error = %v", err)
			}
			intents := payload.BoundaryEffects[testCase.status]
			if len(intents) != 1 || intents[0].Trigger != testCase.trigger {
				t.Fatalf("boundary effects = %#v, want one %q delivery", intents, testCase.trigger)
			}
		})
	}
}

func effectTestRun() Run {
	return Run{
		ID:          "run-effect",
		WorkspaceID: "ws-effect",
		LoopName:    "delivery",
		Inputs:      map[string]any{"channel": "ops"},
	}
}

func decodeRenderedEffectForTest(t *testing.T, raw json.RawMessage) persistedEffectForTest {
	t.Helper()
	var entry persistedEffectForTest
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatalf("json.Unmarshal(effect entry) error = %v", err)
	}
	return entry
}

type persistedEffectForTest struct {
	Kind        string         `json:"kind"`
	With        map[string]any `json:"with"`
	Emit        *dsl.EmitSpec  `json:"emit"`
	RenderError bool           `json:"render_error"`
}
