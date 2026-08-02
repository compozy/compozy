package loop

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/task"
)

func TestStoreFinalizerShouldNormalizeGenerationOutputs(t *testing.T) {
	t.Parallel()

	t.Run("Should persist trimmed node id and status", func(t *testing.T) {
		t.Parallel()

		tx := &generationSnapshotTx{}
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
		if len(tx.args) != 9 {
			t.Fatalf("ExecContext args len = %d, want 9: %#v", len(tx.args), tx.args)
		}
		if got, want := tx.args[0], "loop-run-1"; got != want {
			t.Fatalf("ExecContext loop_run_id arg = %#v, want %q", got, want)
		}
		if got, want := tx.args[2], "worker"; got != want {
			t.Fatalf("ExecContext node_id arg = %#v, want %q", got, want)
		}
		if got, want := tx.args[4], generationOutputSucceeded; got != want {
			t.Fatalf("ExecContext status arg = %#v, want %q", got, want)
		}
		if got := tx.args[8]; got != nil {
			t.Fatalf("ExecContext resolved_runtime arg = %#v, want nil", got)
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
}

func TestGenerationSnapshotLifecycleEventValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a generation-started route", func(t *testing.T) {
		t.Parallel()

		_, err := GenerationSnapshotPayloadFrom(GenerationSnapshotPayload{Events: []GenerationLifecycleEventIntent{{
			Kind:  GenerationLifecycleEventGenerationStarted,
			Route: gate.RouteContinue,
		}}})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("GenerationSnapshotPayloadFrom() error = %v, want ErrValidation", err)
		}
	})
}

type generationSnapshotTx struct {
	args []any
}

func (tx *generationSnapshotTx) ExecContext(_ context.Context, _ string, args ...any) (sql.Result, error) {
	tx.args = append([]any(nil), args...)
	return generationSnapshotResult{}, nil
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

type generationSnapshotResult struct{}

func (generationSnapshotResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (generationSnapshotResult) RowsAffected() (int64, error) {
	return 1, nil
}
