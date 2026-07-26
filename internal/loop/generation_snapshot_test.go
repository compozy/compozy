package loop

import (
	"context"
	"database/sql"
	"testing"

	"github.com/compozy/agh/internal/task"
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
		if len(tx.args) != 8 {
			t.Fatalf("ExecContext args len = %d, want 8: %#v", len(tx.args), tx.args)
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
