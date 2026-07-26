package loop

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/task"
)

// GenerationSnapshotPayload is the loop-owned payload carried by task.GenerationSnapshot.
type GenerationSnapshotPayload struct {
	Outputs     []GenerationOutput     `json:"outputs,omitempty"`
	OutputBlobs []GenerationOutputBlob `json:"output_blobs,omitempty"`
}

// GenerationOutput is one loop_generation_outputs row mutation.
type GenerationOutput struct {
	Generation     int    `json:"generation,omitempty"`
	NodeID         string `json:"node_id"`
	ItemIndex      int    `json:"item_index,omitempty"`
	Status         string `json:"status"`
	OutputRef      string `json:"output_ref,omitempty"`
	TaskRunID      string `json:"task_run_id,omitempty"`
	ChildLoopRunID string `json:"child_loop_run_id,omitempty"`
}

// GenerationOutputBlob is one content-addressed loop output payload required by
// a generation snapshot.
type GenerationOutputBlob struct {
	OutputRef string          `json:"output_ref"`
	Payload   json.RawMessage `json:"payload_json"`
	At        time.Time       `json:"at"`
}

// StoreFinalizer writes loop-owned generation snapshots inside task transactions.
type StoreFinalizer struct{}

var _ task.GenerationStateFinalizer = (*StoreFinalizer)(nil)

// NewStoreFinalizer constructs a loop generation snapshot writer.
func NewStoreFinalizer() *StoreFinalizer {
	return &StoreFinalizer{}
}

// WriteGenerationSnapshot upserts loop_generation_outputs rows in the caller-owned transaction.
func (f *StoreFinalizer) WriteGenerationSnapshot(
	ctx context.Context,
	tx task.Tx,
	snap task.GenerationSnapshot,
) error {
	if tx == nil {
		return fmt.Errorf("%w: transaction is required", ErrValidation)
	}
	loopRunID := strings.TrimSpace(snap.LoopRunID)
	if loopRunID == "" {
		return fmt.Errorf("%w: generation snapshot loop_run_id is required", ErrValidation)
	}
	if snap.Generation <= 0 {
		return fmt.Errorf("%w: generation snapshot generation must be positive", ErrValidation)
	}
	payload, err := normalizeGenerationSnapshotPayload(snap.Payload)
	if err != nil {
		return err
	}
	for _, blob := range payload.OutputBlobs {
		blob = blob.normalized()
		if err := blob.validate(); err != nil {
			return err
		}
		if err := writeGenerationOutputBlob(ctx, tx, blob); err != nil {
			return err
		}
	}
	for _, output := range payload.Outputs {
		output = output.normalized()
		if err := output.validate(); err != nil {
			return err
		}
		_, err := tx.ExecContext(
			ctx,
			`INSERT INTO loop_generation_outputs (
				loop_run_id, generation, node_id, item_index, status, output_ref, task_run_id, child_loop_run_id
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(loop_run_id, generation, node_id, item_index) DO UPDATE SET
				status = excluded.status,
				output_ref = excluded.output_ref,
				task_run_id = excluded.task_run_id,
				child_loop_run_id = excluded.child_loop_run_id`,
			loopRunID,
			snap.Generation,
			output.NodeID,
			output.ItemIndex,
			output.Status,
			sqlNullString(output.OutputRef),
			sqlNullString(output.TaskRunID),
			sqlNullString(output.ChildLoopRunID),
		)
		if err != nil {
			return fmt.Errorf(
				"loop: write generation output %s/%d: %w",
				output.NodeID,
				output.ItemIndex,
				err,
			)
		}
	}
	return nil
}

func (b GenerationOutputBlob) normalized() GenerationOutputBlob {
	b.OutputRef = strings.TrimSpace(b.OutputRef)
	if !b.At.IsZero() {
		b.At = b.At.UTC()
	}
	return b
}

func (b GenerationOutputBlob) validate() error {
	if !OutputRefLooksContentAddressed(b.OutputRef) {
		return fmt.Errorf("%w: output_ref is invalid: %q", ErrValidation, b.OutputRef)
	}
	if len(b.Payload) == 0 {
		return fmt.Errorf("%w: generation output blob payload is required", ErrValidation)
	}
	if !json.Valid(b.Payload) {
		return fmt.Errorf("%w: generation output blob payload must be valid JSON", ErrValidation)
	}
	return nil
}

func writeGenerationOutputBlob(ctx context.Context, tx task.Tx, blob GenerationOutputBlob) error {
	if err := store.UpsertLoopOutputBlob(ctx, tx, blob.OutputRef, blob.Payload, blob.At); err != nil {
		return fmt.Errorf("loop: write generation output blob %q: %w", blob.OutputRef, err)
	}
	return nil
}

func (o GenerationOutput) normalized() GenerationOutput {
	o.NodeID = strings.TrimSpace(o.NodeID)
	o.Status = strings.TrimSpace(o.Status)
	return o
}

func normalizeGenerationSnapshotPayload(value any) (GenerationSnapshotPayload, error) {
	switch typed := value.(type) {
	case nil:
		return GenerationSnapshotPayload{}, nil
	case GenerationSnapshotPayload:
		return typed, nil
	case *GenerationSnapshotPayload:
		if typed == nil {
			return GenerationSnapshotPayload{}, nil
		}
		return *typed, nil
	case []GenerationOutput:
		return GenerationSnapshotPayload{Outputs: typed}, nil
	default:
		return GenerationSnapshotPayload{}, fmt.Errorf(
			"%w: unsupported generation snapshot payload %T",
			ErrValidation,
			value,
		)
	}
}

func (o GenerationOutput) validate() error {
	if strings.TrimSpace(o.NodeID) == "" {
		return fmt.Errorf("%w: generation output node_id is required", ErrValidation)
	}
	if o.ItemIndex < 0 {
		return fmt.Errorf(
			"%w: generation output item_index must be zero or positive",
			ErrValidation,
		)
	}
	switch strings.TrimSpace(o.Status) {
	case generationOutputPending,
		generationOutputEnqueued,
		generationOutputRunning,
		generationOutputAwaitingChild,
		generationOutputControlPending,
		generationOutputAwaitingGoal,
		generationOutputSucceeded,
		generationOutputFailed:
		return nil
	default:
		return fmt.Errorf("%w: generation output status is invalid: %q", ErrValidation, o.Status)
	}
}

func sqlNullString(value string) sql.NullString {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: trimmed, Valid: true}
}
