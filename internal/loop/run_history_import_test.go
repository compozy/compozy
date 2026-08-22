package loop_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/task"
)

func TestRunHistoryImport(t *testing.T) {
	t.Parallel()

	t.Run("Should validate and isolate a complete historical ledger", func(t *testing.T) {
		t.Parallel()

		snapshot := validRunHistorySnapshot(t)
		command, err := loop.NewRunHistoryImport(&snapshot)
		if err != nil {
			t.Fatalf("NewRunHistoryImport() error = %v", err)
		}
		snapshot.Run.Inputs["tasks"].([]any)[0] = "mutated"
		snapshot.Events[0].Payload.(map[string]any)["generation"] = 99
		snapshot.GoalTurns[0].Criteria[0] = 'x'

		isolated := command.Snapshot()
		if !isolated.Run.Historical || isolated.Run.Inputs["tasks"].([]any)[0] != "task-1" {
			t.Fatalf("Snapshot().Run = %#v, want isolated historical input", isolated.Run)
		}
		var payload map[string]any
		if err := json.Unmarshal(isolated.Events[0].Payload.(json.RawMessage), &payload); err != nil {
			t.Fatalf("Unmarshal(event payload) error = %v", err)
		}
		if payload["generation"] != float64(1) || string(isolated.GoalTurns[0].Criteria) != `[]` {
			t.Fatalf("Snapshot() retained caller mutations: payload=%#v turn=%#v", payload, isolated.GoalTurns[0])
		}
	})

	t.Run("Should reject invalid counters and goal turn identities", func(t *testing.T) {
		t.Parallel()

		snapshot := validRunHistorySnapshot(t)
		snapshot.Run.TokensUsed = -1
		if _, err := loop.NewRunHistoryImport(&snapshot); !errors.Is(err, loop.ErrValidation) {
			t.Fatalf("NewRunHistoryImport(negative tokens) error = %v, want ErrValidation", err)
		}
		snapshot = validRunHistorySnapshot(t)
		snapshot.GoalTurns[0].Seq = 2
		if _, err := loop.NewRunHistoryImport(&snapshot); !errors.Is(err, loop.ErrValidation) {
			t.Fatalf("NewRunHistoryImport(gapped goal turns) error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should reject an actor without write authority", func(t *testing.T) {
		t.Parallel()

		snapshot := validRunHistorySnapshot(t)
		snapshot.Actor.Authority = task.Authority{Read: true}
		if _, err := loop.NewRunHistoryImport(&snapshot); !errors.Is(err, task.ErrPermissionDenied) {
			t.Fatalf("NewRunHistoryImport(read-only actor) error = %v, want ErrPermissionDenied", err)
		}
	})
}

func validRunHistorySnapshot(t *testing.T) loop.RunHistorySnapshot {
	t.Helper()
	resolved := compileDefinition(t, validDefinition())
	effective, err := loop.ResolveEffectiveConfig(
		resolved,
		loop.DefaultLoopDefaults(),
		nil,
		loop.LoopConfig{},
	)
	if err != nil {
		t.Fatalf("ResolveEffectiveConfig() error = %v", err)
	}
	definition, digest, err := loop.BuildExecutedDefinitionSnapshot(resolved, effective)
	if err != nil {
		t.Fatalf("BuildExecutedDefinitionSnapshot() error = %v", err)
	}
	createdAt := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	endedAt := createdAt.Add(time.Minute)
	return loop.RunHistorySnapshot{
		Run: loop.Run{
			ID: "run-history", WorkspaceID: "ws-1", LoopName: "valid-loop",
			Status: loop.StatusDone, Generation: 1, CreatedAt: createdAt,
			DefinitionVersion: resolved.DefinitionVersion, DefinitionDigest: digest,
			DefinitionSnapshot: definition, BudgetOnExceeded: dsl.BudgetExceededHalt,
			GoalContextNudgeRatio: 0.8,
			Inputs:                map[string]any{"tasks": []any{"task-1"}},
		},
		Generations: []loop.RunHistoryGeneration{{
			Intent:    loop.GenerationIntent{Generation: 1, Origin: loop.OriginInitial},
			CreatedAt: createdAt,
		}},
		Events: []loop.RunHistoryEvent{{
			Kind:    loop.RunEventGenerationStarted,
			Payload: map[string]any{"generation": 1},
			At:      createdAt,
		}},
		GoalTurns: []loop.RunHistoryGoalTurn{{
			Seq: 1, Generation: 1, NodeID: "worker", Turn: 1,
			SessionID: "session-1", BindingHandle: "goal:worker", BindingEpoch: 1,
			PromptID: "prompt-1", PromptAttempt: 1, StopReason: loop.ActionStopEndTurn,
			VerdictOutcome: gate.VerdictOutcomeApproved,
			BlockingIssues: json.RawMessage(`[]`), Criteria: json.RawMessage(`[]`), Warnings: json.RawMessage(`[]`),
			ActorKind: "agent-session", ActorID: "session-1", StartedAt: createdAt, EndedAt: endedAt,
		}},
		Actor: humanActor(t),
	}
}
