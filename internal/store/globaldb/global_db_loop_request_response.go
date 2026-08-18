package globaldb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	storepkg "github.com/compozy/compozy/internal/store"
)

type requestResponseState struct {
	stored    storedRequest
	run       looppkg.Run
	wait      looppkg.NodeWait
	mutation  looppkg.WaitResumeMutation
	decision  string
	actorKind string
	actorID   string
}

func (g *LoopRepo) respondRequestWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	input looppkg.RespondInput,
	result *looppkg.RespondResult,
) error {
	stored, err := getStoredRequest(ctx, exec, input.WorkspaceID, looppkg.RequestRef{
		RunID: input.RunID, Generation: input.Generation, NodeID: input.NodeID, ItemIndex: input.ItemIndex,
	})
	if err != nil {
		return err
	}
	decision := strings.TrimSpace(input.Decision)
	if decision == "" {
		decision = looppkg.RequestDecisionRespond
	}
	if input.RequestKind != "" && stored.request.Kind != input.RequestKind {
		return fmt.Errorf("%w: request kind changed", looppkg.ErrTransitionConflict)
	}
	actorKind := string(input.Actor.Actor.Kind.Normalize())
	actorID := strings.TrimSpace(input.Actor.Actor.Ref)
	payload, schema, err := requestDecisionPayload(ctx, exec, stored, decision, input.Payload)
	if err != nil {
		return err
	}
	if stored.request.State != looppkg.RequestStatePending {
		return resolvedRequestOutcome(ctx, exec, stored, decision, payload, actorKind, actorID, result)
	}
	if err := validateRequestDecision(stored, decision, schema, payload); err != nil {
		return err
	}
	run, err := nodePauseRun(ctx, exec, input.WorkspaceID, input.RunID)
	if err != nil {
		return err
	}
	mutation := looppkg.WaitResumeMutation{
		WorkspaceID: input.WorkspaceID, RunID: input.RunID,
		Generation: stored.request.Generation, NodeID: input.NodeID, ItemIndex: input.ItemIndex,
		Payload: payload, ClaimedByKind: actorKind, ClaimedByID: actorID,
		AdmissionAttempts: 1, RequestedAt: g.now().UTC(),
	}
	wait, err := loadWaitForResume(ctx, exec, mutation)
	if err != nil {
		return err
	}
	if wait.Kind != looppkg.NodeWaitKindRequest {
		return fmt.Errorf("%w: request wait kind changed", looppkg.ErrTransitionConflict)
	}
	if err := validateWaitResumeCell(ctx, exec, mutation, wait); err != nil {
		return err
	}
	state := requestResponseState{
		stored: stored, run: run, wait: wait, mutation: mutation,
		decision: decision, actorKind: actorKind, actorID: actorID,
	}
	return g.commitRequestResponse(ctx, exec, input, &state, result)
}

func validateRequestDecision(
	stored storedRequest,
	decision string,
	schema json.RawMessage,
	payload json.RawMessage,
) error {
	if !requestAllowsDecision(stored.request.Decisions, decision) {
		return looppkg.NewRequestReasonError(
			looppkg.ReasonCodeRequestValidationFailed,
			fmt.Errorf("%w: decision %q is not allowed", looppkg.ErrRequestValidationFailed, decision),
			map[string]string{watchEventsPayloadDecisionKey: "not allowed"},
		)
	}
	if len(schema) == 0 {
		return nil
	}
	if err := looppkg.ValidateWaitPayload(schema, payload); err != nil {
		return looppkg.NewRequestValidationError(err)
	}
	return nil
}

func (g *LoopRepo) commitRequestResponse(
	ctx context.Context,
	exec taskSQLExecutor,
	input looppkg.RespondInput,
	state *requestResponseState,
	result *looppkg.RespondResult,
) error {
	answerRef := looppkg.OutputRefForPayload(state.mutation.Payload)
	if err := storepkg.UpsertLoopOutputBlob(
		ctx, exec, answerRef, state.mutation.Payload, state.mutation.RequestedAt,
	); err != nil {
		return err
	}
	update, err := exec.ExecContext(ctx, `UPDATE loop_requests SET state = 'answered',
		answered_decision = ?, answered_payload_ref = ?, answered_note = ?, actor_kind = ?,
		actor_id = ?, resolved_at = ? WHERE loop_run_id = ? AND generation = ? AND node_id = ?
		AND item_index = ? AND state = 'pending'`, state.decision, answerRef, strings.TrimSpace(input.Note),
		state.actorKind, state.actorID, state.mutation.RequestedAt, input.RunID, state.stored.request.Generation,
		input.NodeID, input.ItemIndex)
	if err != nil {
		return fmt.Errorf("store: answer Loop request: %w", err)
	}
	if err := requireSingleWaitMutation(update); err != nil {
		return err
	}
	if err := claimRequestDecision(ctx, exec, state.mutation, state.wait, state.stored.request.Kind,
		state.decision, answerRef, input.RejectRoute); err != nil {
		return err
	}
	parkedFor := max(state.mutation.RequestedAt.Sub(state.wait.CreatedAt.UTC()), 0)
	if err := shiftLoopWallClockIfUnparked(ctx, exec, input.RunID, parkedFor); err != nil {
		return err
	}
	if err := appendRequestResponseEvents(ctx, exec, state); err != nil {
		return err
	}
	coordinator, err := g.reserveOrReuseOpenLoopCoordinatorRunWithExecutor(
		ctx, exec, state.run, waitResumeOrigin(state.mutation), state.mutation.RequestedAt,
		waitResumeIdempotencyKey(state.mutation, state.wait.IssuedEpoch),
	)
	if err != nil {
		return err
	}
	state.stored.request.State = looppkg.RequestStateAnswered
	state.stored.request.AnsweredDecision = state.decision
	state.stored.request.ActorKind = state.actorKind
	state.stored.request.ActorID = state.actorID
	resolvedAt := state.mutation.RequestedAt
	state.stored.request.ResolvedAt = &resolvedAt
	*result = looppkg.RespondResult{Request: state.stored.request, Coordinator: &coordinator, Won: true}
	return nil
}

func appendRequestResponseEvents(ctx context.Context, exec taskSQLExecutor, state *requestResponseState) error {
	if err := appendLoopRunEventWithExecutor(ctx, exec, state.run.ID, state.run.WorkspaceID,
		loopRunEventRequestAnswered,
		requestResolutionEventPayload(state.stored, state.decision, state.actorKind, state.actorID),
		state.mutation.RequestedAt); err != nil {
		return err
	}
	return appendLoopRunEventWithExecutor(ctx, exec, state.run.ID, state.run.WorkspaceID,
		loopRunEventNodeWaitResumed, waitResumeEventPayload(state.mutation, state.wait), state.mutation.RequestedAt)
}
