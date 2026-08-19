package loop

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/task"
)

func TestStoreFinalizerShouldNormalizeGenerationOutputs(t *testing.T) {
	t.Parallel()

	t.Run("Should persist trimmed node id and status", func(t *testing.T) {
		t.Parallel()

		tx := &generationSnapshotTx{rowsAffected: 1}
		err := NewStoreFinalizer().WriteGenerationSnapshot(
			context.Background(),
			tx,
			task.GenerationSnapshot{
				LoopRunID:  " loop-run-1 ",
				Generation: 1,
				Payload: GenerationSnapshotPayload{Outputs: []GenerationOutput{{
					NodeID:    " worker ",
					ItemIndex: 0,
					Status:    " " + generationOutputSucceeded + " ",
				}}},
			},
		)
		if err != nil {
			t.Fatalf("WriteGenerationSnapshot() error = %v", err)
		}
		args := tx.calls[0].args
		if len(args) != 14 {
			t.Fatalf("ExecContext args len = %d, want 14: %#v", len(args), args)
		}
		if got, want := args[0], "loop-run-1"; got != want {
			t.Fatalf("ExecContext loop_run_id arg = %#v, want %q", got, want)
		}
		if got, want := args[2], "worker"; got != want {
			t.Fatalf("ExecContext node_id arg = %#v, want %q", got, want)
		}
		if got, want := args[4], generationOutputSucceeded; got != want {
			t.Fatalf("ExecContext status arg = %#v, want %q", got, want)
		}
		if got := args[8]; got != nil {
			t.Fatalf("ExecContext resolved_runtime arg = %#v, want nil", got)
		}
		if got, want := args[9], 1; got != want {
			t.Fatalf("ExecContext attempt arg = %#v, want %d", got, want)
		}
	})

	t.Run("Should persist lifecycle fields behind the observed epoch", func(t *testing.T) {
		t.Parallel()

		nextAttemptAt := time.Date(2026, time.August, 2, 13, 30, 0, 0, time.FixedZone("BRT", -3*60*60))
		firstScheduledAt := nextAttemptAt.Add(-time.Minute)
		expectedEpoch := int64(7)
		tx := &generationSnapshotTx{rowsAffected: 1}
		err := NewStoreFinalizer().WriteGenerationSnapshot(
			context.Background(),
			tx,
			task.GenerationSnapshot{
				LoopRunID:  "loop-run-1",
				Generation: 2,
				Payload: GenerationSnapshotPayload{Outputs: []GenerationOutput{{
					NodeID:           "worker",
					Status:           generationOutputRetrying,
					Attempt:          3,
					NextAttemptAt:    &nextAttemptAt,
					FirstScheduledAt: &firstScheduledAt,
					Epoch:            8,
					ExpectedEpoch:    &expectedEpoch,
				}}},
			},
		)
		if err != nil {
			t.Fatalf("WriteGenerationSnapshot() error = %v", err)
		}
		args := tx.calls[0].args
		if got, want := args[9], 3; got != want {
			t.Fatalf("attempt arg = %#v, want %d", got, want)
		}
		gotNextAttemptAt, ok := args[10].(*time.Time)
		if !ok || !gotNextAttemptAt.Equal(nextAttemptAt.UTC()) {
			t.Fatalf("next_attempt_at arg = %#v, want %v", args[10], nextAttemptAt.UTC())
		}
		gotFirstScheduledAt, ok := args[11].(*time.Time)
		if !ok || !gotFirstScheduledAt.Equal(firstScheduledAt.UTC()) {
			t.Fatalf("first_scheduled_at arg = %#v, want %v", args[11], firstScheduledAt.UTC())
		}
		if got, want := args[12], int64(8); got != want {
			t.Fatalf("epoch arg = %#v, want %d", got, want)
		}
		if got, want := args[13], expectedEpoch; got != want {
			t.Fatalf("expected epoch arg = %#v, want %d", got, want)
		}
	})

	t.Run("Should reject a stale lifecycle write", func(t *testing.T) {
		t.Parallel()

		tx := &generationSnapshotTx{}
		err := NewStoreFinalizer().WriteGenerationSnapshot(
			context.Background(),
			tx,
			task.GenerationSnapshot{
				LoopRunID:  "loop-run-1",
				Generation: 1,
				Payload: GenerationSnapshotPayload{Outputs: []GenerationOutput{{
					NodeID: "worker",
					Status: generationOutputRunning,
					Epoch:  2,
				}}},
			},
		)
		if !errors.Is(err, ErrStaleGenerationOutput) {
			t.Fatalf("WriteGenerationSnapshot() error = %v, want ErrStaleGenerationOutput", err)
		}
	})

	t.Run("Should append an immutable attempt disposition", func(t *testing.T) {
		t.Parallel()

		startedAt := time.Date(2026, time.August, 2, 16, 0, 0, 0, time.UTC)
		endedAt := startedAt.Add(time.Second)
		nextAttemptAt := endedAt.Add(5 * time.Second)
		failureClass := FailureTransport
		tx := &generationSnapshotTx{rowsAffected: 1}
		err := NewStoreFinalizer().WriteGenerationSnapshot(
			context.Background(),
			tx,
			task.GenerationSnapshot{
				LoopRunID:  "loop-run-1",
				Generation: 4,
				Payload: GenerationSnapshotPayload{Attempts: []NodeAttempt{{
					Generation:    4,
					NodeID:        "worker",
					Attempt:       2,
					FailureClass:  &failureClass,
					FailureCode:   "provider_unavailable",
					Cause:         "provider is unavailable",
					Hint:          "retry later",
					Disposition:   AttemptRetried,
					StartedAt:     startedAt,
					EndedAt:       &endedAt,
					NextAttemptAt: &nextAttemptAt,
				}}},
			},
		)
		if err != nil {
			t.Fatalf("WriteGenerationSnapshot() error = %v", err)
		}
		if got, want := len(tx.calls), 1; got != want {
			t.Fatalf("ExecContext call count = %d, want %d", got, want)
		}
		call := tx.calls[0]
		const immutableAttemptConflict = "ON CONFLICT(loop_run_id, generation, node_id, item_index, attempt) DO NOTHING"
		if !strings.Contains(call.query, immutableAttemptConflict) {
			t.Fatalf("attempt SQL = %q, want immutable conflict handling", call.query)
		}
		if got, want := call.args[0], "loop-run-1"; got != want {
			t.Fatalf("attempt loop_run_id arg = %#v, want %q", got, want)
		}
		if got, want := call.args[10], AttemptRetried; got != want {
			t.Fatalf("attempt disposition arg = %#v, want %q", got, want)
		}
	})

	t.Run("Should reject a non-positive node attempt generation", func(t *testing.T) {
		t.Parallel()

		tx := &generationSnapshotTx{rowsAffected: 1}
		err := NewStoreFinalizer().WriteGenerationSnapshot(
			context.Background(),
			tx,
			task.GenerationSnapshot{
				LoopRunID:  "loop-run-1",
				Generation: 1,
				Payload: GenerationSnapshotPayload{Attempts: []NodeAttempt{{
					Generation:  0,
					NodeID:      "worker",
					Attempt:     1,
					Disposition: AttemptSucceeded,
					StartedAt:   time.Now().UTC(),
				}}},
			},
		)
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("WriteGenerationSnapshot() error = %v, want ErrValidation", err)
		}
		if len(tx.calls) != 0 {
			t.Fatalf("ExecContext calls = %d, want no persistence", len(tx.calls))
		}
	})
}

func TestGenerationIntentValidate(t *testing.T) {
	t.Parallel()

	t.Run("Should accept every supported provenance origin", func(t *testing.T) {
		t.Parallel()

		origins := []GenerationOrigin{
			OriginInitial,
			OriginStopWhen,
			OriginReattempt,
			OriginGateRevise,
			OriginGateNextGeneration,
			OriginDoDRetry,
			OriginRatchetRestore,
			OriginRequeue,
			OriginOperatorRerun,
			OriginForkSeed,
		}
		for _, origin := range origins {
			t.Run("Should accept "+string(origin), func(t *testing.T) {
				t.Parallel()

				intent := GenerationIntent{Generation: 2, ParentGeneration: 1, Origin: origin}
				if err := intent.Validate(); err != nil {
					t.Fatalf("GenerationIntent.Validate() error = %v", err)
				}
			})
		}
	})

	t.Run("Should reject a parent generation that is not earlier", func(t *testing.T) {
		t.Parallel()

		intent := GenerationIntent{Generation: 2, ParentGeneration: 2, Origin: OriginGateRevise}
		if err := intent.Validate(); err == nil {
			t.Fatal("GenerationIntent.Validate() error = nil, want non-nil")
		}
	})
}

func TestGenerationSnapshotPayloadFromShouldNormalizeTypedIntents(t *testing.T) {
	t.Parallel()

	t.Run("Should copy typed persistence intents", func(t *testing.T) {
		t.Parallel()

		score := 0.91
		rank := 2
		input := GenerationSnapshotPayload{
			Verdicts: []gate.VerdictIntent{{
				GateID:         " quality ",
				Outcome:        gate.VerdictOutcomeApproved,
				Score:          &score,
				RouteCauseRank: &rank,
				BlockingIssues: json.RawMessage(`[]`),
				Criteria:       json.RawMessage(`[]`),
			}},
			BestUpdate: &gate.BestUpdateIntent{Generation: 2, Score: score},
			GenerationProvenance: &GenerationIntent{
				Generation: 2, ParentGeneration: 1, Origin: OriginGateRevise,
			},
			Events: []GenerationLifecycleEventIntent{{
				Kind:   GenerationLifecycleEventGateVerdict,
				GateID: " quality ",
				Route:  gate.RouteContinue,
			}},
		}
		payload, err := GenerationSnapshotPayloadFrom(input)
		if err != nil {
			t.Fatalf("GenerationSnapshotPayloadFrom() error = %v", err)
		}
		score = 0.10
		rank = 0
		input.BestUpdate.Score = 0.20
		input.GenerationProvenance.Origin = OriginStopWhen
		input.Events[0].GateID = "mutated"
		if got, want := payload.Verdicts[0].GateID, "quality"; got != want {
			t.Fatalf("verdict gate_id = %q, want %q", got, want)
		}
		if got, want := payload.Events[0].GateID, "quality"; got != want {
			t.Fatalf("event gate_id = %q, want %q", got, want)
		}
		if payload.BestUpdate == nil || payload.BestUpdate.Generation != 2 || payload.BestUpdate.Score != 0.91 {
			t.Fatalf("best update = %#v, want generation 2 score 0.91", payload.BestUpdate)
		}
		if payload.GenerationProvenance == nil || payload.GenerationProvenance.Origin != OriginGateRevise {
			t.Fatalf("generation provenance = %#v, want gate revise", payload.GenerationProvenance)
		}
		if payload.Verdicts[0].Score == nil || *payload.Verdicts[0].Score != 0.91 ||
			payload.Verdicts[0].RouteCauseRank == nil || *payload.Verdicts[0].RouteCauseRank != 2 {
			t.Fatalf("verdict pointers = %#v, want preserved copied score and rank", payload.Verdicts[0])
		}
	})

	t.Run("Should reject a gate verdict event without a gate id", func(t *testing.T) {
		t.Parallel()

		_, err := GenerationSnapshotPayloadFrom(GenerationSnapshotPayload{
			Events: []GenerationLifecycleEventIntent{{Kind: GenerationLifecycleEventGateVerdict}},
		})
		if err == nil {
			t.Fatal("GenerationSnapshotPayloadFrom() error = nil, want event validation failure")
		}
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("GenerationSnapshotPayloadFrom() error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should accept approval boundary effects", func(t *testing.T) {
		t.Parallel()

		payload, err := GenerationSnapshotPayloadFrom(GenerationSnapshotPayload{
			BoundaryEffects: map[Status][]RenderedEffectIntent{
				StatusNeedsApproval: {{
					Trigger: EffectTriggerOnFailed, Generation: 1, Entry: json.RawMessage(`{}`),
				}},
			},
		})
		if err != nil {
			t.Fatalf("GenerationSnapshotPayloadFrom(needs approval effects) error = %v", err)
		}
		if len(payload.BoundaryEffects[StatusNeedsApproval]) != 1 {
			t.Fatalf("approval boundary effects = %#v, want one intent", payload.BoundaryEffects)
		}
	})
}

func TestGenerationSnapshotLifecycleEventValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should trim node attention provenance before validation", func(t *testing.T) {
		t.Parallel()

		payload, err := GenerationSnapshotPayloadFrom(GenerationSnapshotPayload{
			Events: []GenerationLifecycleEventIntent{{
				Kind:   GenerationLifecycleEventNodeAttentionFlagged,
				NodeID: " consumer ", ItemIndex: 0, Reason: " producer is parked ",
				AttentionFlag: " dependency_quarantined ", AttentionProducerNodeID: " producer ",
			}},
		})
		if err != nil {
			t.Fatalf("GenerationSnapshotPayloadFrom() error = %v", err)
		}
		event := payload.Events[0]
		if event.NodeID != "consumer" || event.AttentionFlag != "dependency_quarantined" ||
			event.AttentionProducerNodeID != "producer" {
			t.Fatalf("normalized attention event = %#v, want trimmed provenance", event)
		}

		_, err = GenerationSnapshotPayloadFrom(GenerationSnapshotPayload{
			Events: []GenerationLifecycleEventIntent{{
				Kind:   GenerationLifecycleEventNodeAttentionFlagged,
				NodeID: "consumer", ItemIndex: 0, Reason: "producer is parked",
				AttentionFlag: " ", AttentionProducerNodeID: " producer ",
			}},
		})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("GenerationSnapshotPayloadFrom(blank attention flag) error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should reject a generation-started route", func(t *testing.T) {
		t.Parallel()

		_, err := GenerationSnapshotPayloadFrom(GenerationSnapshotPayload{Events: []GenerationLifecycleEventIntent{{
			Kind:  GenerationLifecycleEventGenerationStarted,
			Route: gate.RouteContinue,
		}}})
		const wantDetail = "route"
		if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), wantDetail) {
			t.Fatalf(
				"GenerationSnapshotPayloadFrom() error = %v, want ErrValidation with detail %q",
				err,
				wantDetail,
			)
		}
	})
}

type generationSnapshotTx struct {
	calls        []generationSnapshotCall
	rowsAffected int64
}

type generationSnapshotCall struct {
	query string
	args  []any
}

func (tx *generationSnapshotTx) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	tx.calls = append(tx.calls, generationSnapshotCall{
		query: query,
		args:  append([]any(nil), args...),
	})
	return generationSnapshotResult{rowsAffected: tx.rowsAffected}, nil
}

func (tx *generationSnapshotTx) QueryContext(
	context.Context,
	string,
	...any,
) (*sql.Rows, error) {
	return nil, nil
}

func (tx *generationSnapshotTx) QueryRowContext(context.Context, string, ...any) *sql.Row {
	return nil
}

type generationSnapshotResult struct {
	rowsAffected int64
}

func (generationSnapshotResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (r generationSnapshotResult) RowsAffected() (int64, error) {
	return r.rowsAffected, nil
}
