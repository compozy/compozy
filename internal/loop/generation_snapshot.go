package loop

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/task"
)

// GenerationSnapshotPayload is the loop-owned payload carried by task.GenerationSnapshot.
type GenerationSnapshotPayload struct {
	Outputs              []GenerationOutput                `json:"outputs,omitempty"`
	Attempts             []NodeAttempt                     `json:"attempts,omitempty"`
	Controls             []NodeControlMutation             `json:"controls,omitempty"`
	Waits                []NodeWaitIntent                  `json:"waits,omitempty"`
	Requests             []RequestIntent                   `json:"requests,omitempty"`
	OutputBlobs          []GenerationOutputBlob            `json:"output_blobs,omitempty"`
	Verdicts             []gate.VerdictIntent              `json:"verdicts,omitempty"`
	BestUpdate           *gate.BestUpdateIntent            `json:"best_update,omitempty"`
	GenerationProvenance *GenerationIntent                 `json:"generation_provenance,omitempty"`
	Events               []GenerationLifecycleEventIntent  `json:"events,omitempty"`
	BoundaryEffects      map[Status][]RenderedEffectIntent `json:"boundary_effects,omitempty"`
}

// GenerationOutput is one loop_generation_outputs row mutation.
type GenerationOutput struct {
	Generation       int              `json:"generation,omitempty"`
	NodeID           string           `json:"node_id"`
	ItemIndex        int              `json:"item_index,omitempty"`
	Status           string           `json:"status"`
	OutputRef        string           `json:"output_ref,omitempty"`
	TaskRunID        string           `json:"task_run_id,omitempty"`
	ChildLoopRunID   string           `json:"child_loop_run_id,omitempty"`
	ResolvedRuntime  *ResolvedRuntime `json:"resolved_runtime,omitempty"`
	Attempt          int              `json:"attempt,omitempty"`
	NextAttemptAt    *time.Time       `json:"next_attempt_at,omitempty"`
	FirstScheduledAt *time.Time       `json:"first_scheduled_at,omitempty"`
	Epoch            int64            `json:"epoch,omitempty"`
	// SessionID is the ACP session bound to the cell's task run; it is a
	// read-model join and never part of snapshot state.
	SessionID string `json:"-"`
	// ExpectedEpoch is the cell epoch observed by the planner. It is used only
	// for compare-and-swap and is never stored as domain state.
	ExpectedEpoch *int64 `json:"expected_epoch,omitempty"`
	// runtimePayload is hydrated in-process and intentionally omitted from snapshots.
	runtimePayload json.RawMessage
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
	payload, err := GenerationSnapshotPayloadFrom(snap.Payload)
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
		resolvedRuntime, err := resolvedRuntimeSQLValue(output.ResolvedRuntime)
		if err != nil {
			return err
		}
		if err := writeGenerationOutput(ctx, tx, loopRunID, snap.Generation, output, resolvedRuntime); err != nil {
			return err
		}
	}
	for _, attempt := range payload.Attempts {
		if err := writeGenerationAttempt(ctx, tx, loopRunID, snap.Generation, attempt); err != nil {
			return err
		}
	}
	for _, control := range payload.Controls {
		if err := writeNodeControlMutation(ctx, tx, loopRunID, control); err != nil {
			return err
		}
		if control.Kind == NodeControlMutationQuarantine {
			if err := markQuarantinedNodeTasks(ctx, tx, loopRunID, snap.Generation, payload, control); err != nil {
				return err
			}
		}
	}
	for _, wait := range payload.Waits {
		if err := writeNodeWaitIntent(ctx, tx, loopRunID, snap.Generation, wait); err != nil {
			return err
		}
	}
	for _, request := range payload.Requests {
		if err := writeRequestIntent(ctx, tx, loopRunID, snap.Generation, request); err != nil {
			return err
		}
	}
	return nil
}

func writeRequestIntent(
	ctx context.Context,
	tx task.Tx,
	loopRunID string,
	generation int,
	intent RequestIntent,
) error {
	intent = intent.normalized()
	if err := intent.validate(); err != nil {
		return err
	}
	contextRef := OutputRefForPayload(intent.Context)
	if err := store.UpsertLoopOutputBlob(ctx, tx, contextRef, intent.Context, intent.OpenedAt); err != nil {
		return fmt.Errorf("loop: persist request context: %w", err)
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO loop_requests (
		workspace_id, loop_run_id, generation, node_id, item_index, kind, state,
		prompt, context_preview_json, context_ref, answer_schema_json, decisions_json,
		respond_schema_json, opened_at, expires_at
	) VALUES (?, ?, ?, ?, ?, ?, 'pending', ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(loop_run_id, generation, node_id, item_index) DO NOTHING`,
		intent.WorkspaceID, loopRunID, generation, intent.NodeID, intent.ItemIndex, intent.Kind,
		intent.Prompt, string(intent.ContextPreview), contextRef, string(intent.AnswerSchema),
		string(intent.Decisions), string(intent.AnswerSchema), intent.OpenedAt, intent.ExpiresAt)
	if err != nil {
		return fmt.Errorf("loop: write request %s/%d: %w", intent.NodeID, intent.ItemIndex, err)
	}
	return nil
}

func writeNodeWaitIntent(
	ctx context.Context,
	tx task.Tx,
	loopRunID string,
	generation int,
	intent NodeWaitIntent,
) error {
	intent = intent.normalized()
	if err := intent.validate(); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO loop_node_waits (
		loop_run_id, generation, node_id, item_index, kind, resume_at, next_escalation_at,
		claim_state, claimed_by_kind, claimed_by_id, claimed_at, expect_json,
		ahead_payload_json, issued_epoch, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(loop_run_id, generation, node_id, item_index) DO UPDATE SET
		kind = excluded.kind, resume_at = excluded.resume_at,
		next_escalation_at = excluded.next_escalation_at, claim_state = excluded.claim_state,
		claimed_by_kind = excluded.claimed_by_kind, claimed_by_id = excluded.claimed_by_id,
		claimed_at = excluded.claimed_at,
		admission_failures = 0, expect_json = excluded.expect_json,
		ahead_payload_json = excluded.ahead_payload_json, issued_epoch = excluded.issued_epoch,
		created_at = excluded.created_at
	WHERE loop_node_waits.issued_epoch < excluded.issued_epoch`,
		loopRunID, generation, intent.NodeID, intent.ItemIndex, intent.Kind, intent.ResumeAt,
		intent.NextEscalationAt, intent.ClaimState, sqlNullString(intent.ClaimedByKind),
		sqlNullString(intent.ClaimedByID), intent.ClaimedAt, sqlNullRawJSON(intent.Expect),
		sqlNullRawJSON(intent.AheadPayload), intent.IssuedEpoch, intent.CreatedAt)
	if err != nil {
		return fmt.Errorf("loop: write node wait %s/%d: %w", intent.NodeID, intent.ItemIndex, err)
	}
	return nil
}

func sqlNullRawJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return string(raw)
}

func writeGenerationOutput(
	ctx context.Context,
	tx task.Tx,
	loopRunID string,
	generation int,
	output GenerationOutput,
	resolvedRuntime any,
) error {
	expectedEpoch := output.Epoch
	if output.ExpectedEpoch != nil {
		expectedEpoch = *output.ExpectedEpoch
	}
	result, err := tx.ExecContext(
		ctx,
		`INSERT INTO loop_generation_outputs (
			loop_run_id, generation, node_id, item_index, status, output_ref, task_run_id,
			child_loop_run_id, resolved_runtime_json, attempt, next_attempt_at,
			first_scheduled_at, epoch
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(loop_run_id, generation, node_id, item_index) DO UPDATE SET
			status = excluded.status,
			output_ref = excluded.output_ref,
			task_run_id = excluded.task_run_id,
			child_loop_run_id = excluded.child_loop_run_id,
			resolved_runtime_json = COALESCE(
				excluded.resolved_runtime_json,
				loop_generation_outputs.resolved_runtime_json
			),
			attempt = excluded.attempt,
			next_attempt_at = excluded.next_attempt_at,
			first_scheduled_at = COALESCE(
				loop_generation_outputs.first_scheduled_at,
				excluded.first_scheduled_at
			),
			epoch = excluded.epoch
		WHERE loop_generation_outputs.epoch = ?`,
		loopRunID,
		generation,
		output.NodeID,
		output.ItemIndex,
		output.Status,
		sqlNullString(output.OutputRef),
		sqlNullString(output.TaskRunID),
		sqlNullString(output.ChildLoopRunID),
		resolvedRuntime,
		output.Attempt,
		output.NextAttemptAt,
		output.FirstScheduledAt,
		output.Epoch,
		expectedEpoch,
	)
	if err != nil {
		return fmt.Errorf("loop: write generation output %s/%d: %w", output.NodeID, output.ItemIndex, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("loop: inspect generation output %s/%d write: %w", output.NodeID, output.ItemIndex, err)
	}
	if affected == 0 {
		return fmt.Errorf(
			"%w: output %s/%d expected epoch %d",
			ErrStaleGenerationOutput,
			output.NodeID,
			output.ItemIndex,
			expectedEpoch,
		)
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
	if o.ResolvedRuntime != nil {
		normalized := normalizeResolvedRuntime(*o.ResolvedRuntime)
		o.ResolvedRuntime = &normalized
	}
	if o.Attempt == 0 {
		o.Attempt = 1
	}
	if o.NextAttemptAt != nil {
		value := o.NextAttemptAt.UTC()
		o.NextAttemptAt = &value
	}
	if o.FirstScheduledAt != nil {
		value := o.FirstScheduledAt.UTC()
		o.FirstScheduledAt = &value
	}
	if o.ExpectedEpoch != nil {
		value := *o.ExpectedEpoch
		o.ExpectedEpoch = &value
	}
	return o
}

func resolvedRuntimeSQLValue(runtime *ResolvedRuntime) (any, error) {
	if runtime == nil {
		return nil, nil
	}
	data, err := json.Marshal(runtime)
	if err != nil {
		return nil, fmt.Errorf("loop: marshal resolved runtime: %w", err)
	}
	return string(data), nil
}

// GenerationSnapshotPayloadFrom normalizes the loop-owned payload of a generation snapshot.
func GenerationSnapshotPayloadFrom(value any) (GenerationSnapshotPayload, error) {
	var payload GenerationSnapshotPayload
	switch typed := value.(type) {
	case nil:
		return payload, nil
	case GenerationSnapshotPayload:
		payload = typed
	case *GenerationSnapshotPayload:
		if typed == nil {
			return payload, nil
		}
		payload = *typed
	default:
		return GenerationSnapshotPayload{}, fmt.Errorf(
			"%w: unsupported generation snapshot payload %T",
			ErrValidation,
			value,
		)
	}
	return normalizeGenerationSnapshotIntents(payload)
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
	if o.Attempt < 1 {
		return fmt.Errorf("%w: generation output attempt must be positive", ErrValidation)
	}
	if o.Epoch < 0 {
		return fmt.Errorf("%w: generation output epoch must be non-negative", ErrValidation)
	}
	if o.ExpectedEpoch != nil && *o.ExpectedEpoch < 0 {
		return fmt.Errorf("%w: generation output expected_epoch must be non-negative", ErrValidation)
	}
	if o.Status == generationOutputRetrying && o.NextAttemptAt == nil {
		return fmt.Errorf("%w: retrying generation output requires next_attempt_at", ErrValidation)
	}
	switch strings.TrimSpace(o.Status) {
	case generationOutputPending,
		generationOutputEnqueued,
		generationOutputRunning,
		generationOutputRetrying,
		generationOutputWaiting,
		generationOutputPaused,
		generationOutputAwaitingChild,
		generationOutputControlPending,
		generationOutputAwaitingGoal,
		generationOutputSucceeded,
		generationOutputFailed,
		generationOutputCanceled,
		generationOutputQuarantined:
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
