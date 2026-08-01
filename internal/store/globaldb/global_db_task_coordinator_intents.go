package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/gate"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func applyCoordinatorGenerationSnapshotIntentsWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	snapshot taskpkg.GenerationSnapshot,
	at time.Time,
) error {
	payload, err := looppkg.GenerationSnapshotPayloadFrom(snapshot.Payload)
	if err != nil {
		return err
	}
	if snapshot.Generation <= 0 {
		return fmt.Errorf("%w: generation snapshot generation must be positive", looppkg.ErrValidation)
	}
	var provenance looppkg.GenerationIntent
	if generationSnapshotRequiresProvenance(payload, snapshot.Generation, run.Generation) {
		provenance, err = persistGenerationProvenanceWithExecutor(ctx, exec, run, snapshot, payload, at)
		if err != nil {
			return err
		}
	}
	if err := persistGenerationVerdictsWithExecutor(
		ctx,
		exec,
		run,
		snapshot.Generation,
		payload.Verdicts,
		at,
	); err != nil {
		return err
	}
	if err := persistGenerationBestUpdateWithExecutor(
		ctx,
		exec,
		run,
		snapshot.Generation,
		payload.BestUpdate,
	); err != nil {
		return err
	}
	return appendGenerationLifecycleEventsWithExecutor(ctx, exec, run, snapshot.Generation, provenance, payload, at)
}

func generationSnapshotRequiresProvenance(
	payload looppkg.GenerationSnapshotPayload,
	generation int,
	currentGeneration int,
) bool {
	if payload.GenerationProvenance != nil || generation > currentGeneration {
		return true
	}
	for _, event := range payload.Events {
		if event.Kind == looppkg.GenerationLifecycleEventGenerationStarted {
			return true
		}
	}
	return false
}

func persistGenerationProvenanceWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	snapshot taskpkg.GenerationSnapshot,
	payload looppkg.GenerationSnapshotPayload,
	at time.Time,
) (looppkg.GenerationIntent, error) {
	if payload.GenerationProvenance != nil {
		provenance := *payload.GenerationProvenance
		if provenance.Generation != int64(snapshot.Generation) {
			return looppkg.GenerationIntent{}, fmt.Errorf(
				"%w: generation provenance %d does not match snapshot generation %d",
				looppkg.ErrValidation,
				provenance.Generation,
				snapshot.Generation,
			)
		}
		if err := insertLoopGenerationWithExecutor(ctx, exec, run.ID, provenance, at); err != nil {
			return looppkg.GenerationIntent{}, err
		}
		return provenance, nil
	}
	return getLoopGenerationIntentWithExecutor(ctx, exec, run.ID, snapshot.Generation)
}

func getLoopGenerationIntentWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	generation int,
) (looppkg.GenerationIntent, error) {
	var parentGeneration int64
	var origin string
	err := exec.QueryRowContext(
		ctx,
		`SELECT parent_generation, origin
		 FROM loop_generations
		 WHERE loop_run_id = ? AND generation = ?`,
		string(runID),
		generation,
	).Scan(&parentGeneration, &origin)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return looppkg.GenerationIntent{}, fmt.Errorf(
				"%w: generation provenance is required for loop run %q generation %d",
				looppkg.ErrValidation,
				runID,
				generation,
			)
		}
		return looppkg.GenerationIntent{}, fmt.Errorf(
			"store: get generation provenance for loop run %q generation %d: %w",
			runID,
			generation,
			err,
		)
	}
	provenance := looppkg.GenerationIntent{
		Generation:       int64(generation),
		ParentGeneration: parentGeneration,
		Origin:           looppkg.GenerationOrigin(origin),
	}
	if err := provenance.Validate(); err != nil {
		return looppkg.GenerationIntent{}, fmt.Errorf("store: validate persisted generation provenance: %w", err)
	}
	return provenance, nil
}

func persistGenerationVerdictsWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	generation int,
	verdicts []gate.VerdictIntent,
	at time.Time,
) error {
	for _, verdict := range verdicts {
		if err := insertLoopGateVerdictWithExecutor(ctx, exec, run.ID, generation, verdict, at); err != nil {
			return err
		}
	}
	return nil
}

func persistGenerationBestUpdateWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	generation int,
	best *gate.BestUpdateIntent,
) error {
	if best == nil {
		return nil
	}
	if best.Generation != int64(generation) {
		return fmt.Errorf(
			"%w: generation best update %d does not match snapshot generation %d",
			looppkg.ErrValidation,
			best.Generation,
			generation,
		)
	}
	return updateLoopRunBestWithExecutor(ctx, exec, run.WorkspaceID, run.ID, &best.Generation, &best.Score)
}

func appendGenerationLifecycleEventsWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	generation int,
	provenance looppkg.GenerationIntent,
	payload looppkg.GenerationSnapshotPayload,
	at time.Time,
) error {
	hasGenerationStarted := false
	for _, event := range payload.Events {
		switch event.Kind {
		case looppkg.GenerationLifecycleEventGenerationStarted:
			hasGenerationStarted = true
			if err := appendLoopGenerationStartedEventWithExecutor(ctx, exec, run, provenance, at); err != nil {
				return err
			}
		case looppkg.GenerationLifecycleEventGateVerdict:
			verdict, found := generationVerdictByGateInstance(
				payload.Verdicts,
				event.GateID,
				event.ItemIndex,
			)
			if !found {
				return fmt.Errorf(
					"%w: gate_verdict event references unknown gate %q item %d",
					looppkg.ErrValidation,
					event.GateID,
					event.ItemIndex,
				)
			}
			if err := appendLoopGateVerdictEventWithExecutor(
				ctx,
				exec,
				run,
				generation,
				verdict,
				event,
				at,
			); err != nil {
				return err
			}
		}
	}
	if !hasGenerationStarted && generation > run.Generation {
		if err := appendLoopGenerationStartedEventWithExecutor(ctx, exec, run, provenance, at); err != nil {
			return err
		}
	}
	return nil
}

func generationVerdictByGateInstance(
	verdicts []gate.VerdictIntent,
	gateID string,
	itemIndex int,
) (gate.VerdictIntent, bool) {
	trimmedGateID := strings.TrimSpace(gateID)
	for _, verdict := range verdicts {
		if verdict.GateID == trimmedGateID && verdict.ItemIndex == itemIndex {
			return verdict, true
		}
	}
	return gate.VerdictIntent{}, false
}
