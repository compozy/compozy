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
	if err := applyStrategyCancellationsWithExecutor(
		ctx, exec, run, snapshot.Generation, payload.StrategyCancellations,
	); err != nil {
		return err
	}
	if err := appendGenerationLifecycleEventsWithExecutor(
		ctx, exec, run, snapshot.Generation, provenance, payload, at,
	); err != nil {
		return err
	}
	return appendRequestOpenedEventsWithExecutor(ctx, exec, run, snapshot.Generation, payload.Requests)
}

func appendRequestOpenedEventsWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	generation int,
	requests []looppkg.RequestIntent,
) error {
	for _, request := range requests {
		payload := map[string]any{
			loopRunEventPayloadKeyGeneration: generation,
			loopRunEventPayloadKeyNodeID:     request.NodeID,
			loopRunEventPayloadKeyItemIndex:  request.ItemIndex,
			loopRunEventPayloadKeyKind:       request.Kind,
			loopRunEventPayloadKeyPrompt:     request.Prompt,
			"context":                        request.ContextPreview,
			"expect":                         request.AnswerSchema,
			"decisions":                      request.Decisions,
			"agents":                         request.Agents,
			loopRunEventPayloadKeyExpiresAt:  request.ExpiresAt,
		}
		if err := appendLoopRunEventWithExecutor(
			ctx, exec, run.ID, run.WorkspaceID, loopRunEventRequestOpened, payload, request.OpenedAt,
		); err != nil {
			return err
		}
	}
	return nil
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
		if event.Kind == looppkg.GenerationLifecycleEventGenerationStarted {
			hasGenerationStarted = true
		}
		if err := appendGenerationLifecycleEventWithExecutor(
			ctx, exec, run, generation, provenance, payload.Verdicts, event, at,
		); err != nil {
			return err
		}
	}
	if !hasGenerationStarted && generation > run.Generation {
		if err := appendLoopGenerationStartedEventWithExecutor(ctx, exec, run, provenance, at); err != nil {
			return err
		}
	}
	return nil
}

func appendGenerationLifecycleEventWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	generation int,
	provenance looppkg.GenerationIntent,
	verdicts []gate.VerdictIntent,
	event looppkg.GenerationLifecycleEventIntent,
	at time.Time,
) error {
	switch event.Kind {
	case looppkg.GenerationLifecycleEventGenerationStarted:
		return appendLoopGenerationStartedEventWithExecutor(ctx, exec, run, provenance, at)
	case looppkg.GenerationLifecycleEventGateVerdict:
		return appendGenerationGateVerdictEvent(ctx, exec, run, generation, verdicts, event, at)
	case looppkg.GenerationLifecycleEventNodeRetryScheduled:
		return appendRetryScheduledEffectEventWithExecutor(ctx, exec, run, generation, event, at)
	case looppkg.GenerationLifecycleEventNodeSucceeded,
		looppkg.GenerationLifecycleEventNodeFailed,
		looppkg.GenerationLifecycleEventNodeCanceled:
		return appendNodeOutcomeEffectEventWithExecutor(ctx, exec, run, generation, event, at)
	case looppkg.GenerationLifecycleEventNodeQuarantined:
		return appendNodeQuarantinedEffectEventWithExecutor(ctx, exec, run, generation, event, at)
	case looppkg.GenerationLifecycleEventNodeAttentionFlagged:
		return appendLoopRunEventWithExecutor(ctx, exec, run.ID, run.WorkspaceID,
			loopRunEventNodeAttentionFlagged, map[string]any{
				loopRunEventPayloadKeyGeneration:    generation,
				loopRunEventPayloadKeyNodeID:        event.NodeID,
				loopRunEventPayloadKeyItemIndex:     event.ItemIndex,
				loopRunEventPayloadKeyAttentionFlag: event.AttentionFlag,
				"producer_node_id":                  event.AttentionProducerNodeID,
				loopRunEventPayloadKeyReason:        event.Reason,
			}, at)
	case looppkg.GenerationLifecycleEventNodePaused:
		return appendGenerationNodePausedEvent(ctx, exec, run, generation, event, at)
	case looppkg.GenerationLifecycleEventNodeWaitStarted:
		return appendGenerationNodeWaitStartedEvent(ctx, exec, run, generation, event, at)
	case looppkg.GenerationLifecycleEventNodeWaitResumed:
		return appendGenerationNodeWaitResumedEvent(ctx, exec, run, generation, event, at)
	case looppkg.GenerationLifecycleEventTargetBreakerTransition:
		return appendTargetBreakerTransitionEventWithExecutor(ctx, exec, run, generation, event, at)
	case looppkg.GenerationLifecycleEventPredicateDiagnostic:
		return appendLoopRunEventWithExecutor(ctx, exec, run.ID, run.WorkspaceID,
			loopRunEventPredicateDiagnostic, map[string]any{
				loopRunEventPayloadKeyGeneration: generation,
				"predicate":                      event.Predicate,
				"code":                           event.DiagnosticCode,
				loopRunEventPayloadKeyReason:     event.Reason,
				"cost":                           event.Cost,
				"cost_limit":                     event.CostLimit,
				"warning":                        event.Warning,
			}, at)
	case looppkg.GenerationLifecycleEventRouteTaken:
		return appendGenerationRouteTakenEvent(ctx, exec, run, generation, event, at)
	case looppkg.GenerationLifecycleEventBranchPruned:
		return appendLoopRunEventWithExecutor(ctx, exec, run.ID, run.WorkspaceID,
			loopRunEventBranchPruned, map[string]any{
				loopRunEventPayloadKeyGeneration: generation,
				loopRunEventPayloadKeyNodeID:     event.NodeID,
				"item_indexes":                   event.ItemIndexes,
				loopRunEventPayloadKeyReason:     event.Reason,
			}, at)
	default:
		return nil
	}
}

func appendGenerationRouteTakenEvent(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	generation int,
	event looppkg.GenerationLifecycleEventIntent,
	at time.Time,
) error {
	payload := map[string]any{
		loopRunEventPayloadKeyGeneration: generation,
		loopRunEventPayloadKeyNodeID:     event.NodeID,
		loopRunEventPayloadKeyItemIndex:  event.ItemIndex,
		loopRunEventPayloadKeyRoute:      event.SelectedRoute,
		loopRunEventPayloadKeyCause:      event.Reason,
	}
	if event.MatchedWhen != "" {
		payload[loopRunEventPayloadKeyMatchedWhen] = event.MatchedWhen
	}
	if event.DefaultRoute {
		payload[loopRunEventPayloadKeyDefault] = true
	}
	return appendLoopRunEventWithExecutor(
		ctx, exec, run.ID, run.WorkspaceID, loopRunEventRouteTaken, payload, at,
	)
}

func appendGenerationGateVerdictEvent(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	generation int,
	verdicts []gate.VerdictIntent,
	event looppkg.GenerationLifecycleEventIntent,
	at time.Time,
) error {
	verdict, found := generationVerdictByGateInstance(verdicts, event.GateID, event.ItemIndex)
	if !found {
		return fmt.Errorf(
			"%w: gate_verdict event references unknown gate %q item %d",
			looppkg.ErrValidation, event.GateID, event.ItemIndex,
		)
	}
	return appendLoopGateVerdictEventWithExecutor(ctx, exec, run, generation, verdict, event, at)
}

func appendGenerationNodePausedEvent(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	generation int,
	event looppkg.GenerationLifecycleEventIntent,
	at time.Time,
) error {
	return appendLoopRunEventWithExecutor(ctx, exec, run.ID, run.WorkspaceID, loopRunEventNodePaused,
		map[string]any{
			loopRunEventPayloadKeyGeneration: generation,
			loopRunEventPayloadKeyNodeID:     event.NodeID,
			loopRunEventPayloadKeyItemIndex:  event.ItemIndex,
			watchEventsPayloadAttemptKey:     event.Attempt,
			loopRunEventPayloadKeyActorKind:  event.ActorKind,
			loopRunEventPayloadKeyActorID:    event.ActorID,
			loopRunEventPayloadKeyReason:     event.Reason,
			"rule_id":                        event.RuleID,
		}, at)
}

func appendGenerationNodeWaitStartedEvent(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	generation int,
	event looppkg.GenerationLifecycleEventIntent,
	at time.Time,
) error {
	return appendLoopRunEventWithExecutor(ctx, exec, run.ID, run.WorkspaceID,
		loopRunEventNodeWaitStarted, map[string]any{
			loopRunEventPayloadKeyGeneration:  generation,
			loopRunEventPayloadKeyNodeID:      event.NodeID,
			loopRunEventPayloadKeyItemIndex:   event.ItemIndex,
			loopRunEventPayloadKeyIssuedEpoch: event.IssuedEpoch,
			loopRunEventPayloadKeyWaitKind:    event.WaitKind,
			"resume_at":                       event.NextAttemptAt,
			"ahead_arrival":                   event.AheadArrival,
			"ahead_cursors":                   event.AheadCursors,
		}, at)
}

func appendGenerationNodeWaitResumedEvent(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	generation int,
	event looppkg.GenerationLifecycleEventIntent,
	at time.Time,
) error {
	return appendLoopRunEventWithExecutor(ctx, exec, run.ID, run.WorkspaceID,
		loopRunEventNodeWaitResumed, map[string]any{
			loopRunEventPayloadKeyGeneration:  generation,
			loopRunEventPayloadKeyNodeID:      event.NodeID,
			loopRunEventPayloadKeyItemIndex:   event.ItemIndex,
			loopRunEventPayloadKeyIssuedEpoch: event.IssuedEpoch,
			loopRunEventPayloadKeyActorKind:   event.ActorKind,
			loopRunEventPayloadKeyActorID:     event.ActorID,
			loopRunEventPayloadKeyWaitKind:    event.WaitKind,
		}, at)
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
