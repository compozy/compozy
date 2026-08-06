package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	looppkg "github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (s *daemonLoopAPIService) CancelLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	actor taskpkg.ActorContext,
) (contract.LoopMutationResponse, error) {
	return s.mutateLoopRunCancellation(ctx, workspaceID, runID, false, actor)
}

func (s *daemonLoopAPIService) KillLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	actor taskpkg.ActorContext,
) (contract.LoopMutationResponse, error) {
	return s.mutateLoopRunCancellation(ctx, workspaceID, runID, true, actor)
}

func (s *daemonLoopAPIService) PauseLoopNode(
	ctx context.Context,
	workspaceID string,
	runID string,
	nodeID string,
	req contract.LoopNodePauseRequest,
	actor taskpkg.ActorContext,
) (contract.LoopMutationResponse, error) {
	ws, normalizedRunID, normalizedNodeID, err := normalizeLoopNodeIdentity(workspaceID, runID, nodeID)
	if err != nil {
		return contract.LoopMutationResponse{}, err
	}
	lifecycle, err := s.nodeLifecycleService()
	if err != nil {
		return contract.LoopMutationResponse{}, err
	}
	result, err := lifecycle.PauseNode(
		ctx,
		ws,
		normalizedRunID,
		normalizedNodeID,
		looppkg.NodePauseMode(strings.TrimSpace(req.Mode)),
		req.Reason,
		actor,
	)
	if err != nil {
		return contract.LoopMutationResponse{}, err
	}
	control := loopNodeControlPayload(result.Control)
	return loopNodeMutationResponse(normalizedRunID, normalizedNodeID, &control), nil
}

func (s *daemonLoopAPIService) ResumeLoopNode(
	ctx context.Context,
	workspaceID string,
	runID string,
	nodeID string,
	req contract.LoopNodeResumeRequest,
	actor taskpkg.ActorContext,
) (contract.LoopMutationResponse, error) {
	ws, normalizedRunID, normalizedNodeID, err := normalizeLoopNodeIdentity(workspaceID, runID, nodeID)
	if err != nil {
		return contract.LoopMutationResponse{}, err
	}
	lifecycle, err := s.nodeLifecycleService()
	if err != nil {
		return contract.LoopMutationResponse{}, err
	}
	if len(req.Payload) > 0 {
		itemIndex := 0
		if req.ItemIndex != nil {
			itemIndex = *req.ItemIndex
		}
		if _, err := lifecycle.ResumeNodeWait(
			ctx, ws, normalizedRunID, normalizedNodeID, itemIndex, req.Payload, actor,
		); err != nil {
			return contract.LoopMutationResponse{}, err
		}
		return loopNodeMutationResponse(normalizedRunID, normalizedNodeID, nil), nil
	}
	result, err := lifecycle.ResumeNode(
		ctx,
		ws,
		normalizedRunID,
		normalizedNodeID,
		looppkg.NodeResumeMode(strings.TrimSpace(req.Mode)),
		actor,
	)
	if err != nil {
		return contract.LoopMutationResponse{}, err
	}
	control := loopNodeControlPayload(result.Control)
	return loopNodeMutationResponse(normalizedRunID, normalizedNodeID, &control), nil
}

func (s *daemonLoopAPIService) CancelLoopNode(
	ctx context.Context,
	workspaceID string,
	runID string,
	nodeID string,
	req contract.LoopNodeMutationRequest,
	actor taskpkg.ActorContext,
) (contract.LoopMutationResponse, error) {
	return s.mutateLoopNodeCancellation(ctx, workspaceID, runID, nodeID, req.Reason, false, actor)
}

func (s *daemonLoopAPIService) KillLoopNode(
	ctx context.Context,
	workspaceID string,
	runID string,
	nodeID string,
	req contract.LoopNodeMutationRequest,
	actor taskpkg.ActorContext,
) (contract.LoopMutationResponse, error) {
	return s.mutateLoopNodeCancellation(ctx, workspaceID, runID, nodeID, req.Reason, true, actor)
}

func (s *daemonLoopAPIService) RequeueLoopNode(
	ctx context.Context,
	workspaceID string,
	runID string,
	nodeID string,
	req contract.LoopNodeMutationRequest,
	actor taskpkg.ActorContext,
) (contract.LoopMutationResponse, error) {
	ws, normalizedRunID, normalizedNodeID, err := normalizeLoopNodeIdentity(workspaceID, runID, nodeID)
	if err != nil {
		return contract.LoopMutationResponse{}, err
	}
	lifecycle, err := s.nodeLifecycleService()
	if err != nil {
		return contract.LoopMutationResponse{}, err
	}
	result, err := lifecycle.RequeueNode(ctx, ws, normalizedRunID, normalizedNodeID, req.Reason, actor)
	if err != nil {
		return contract.LoopMutationResponse{}, err
	}
	control := loopNodeControlPayload(result.Control)
	return loopNodeMutationResponse(normalizedRunID, normalizedNodeID, &control), nil
}

func (s *daemonLoopAPIService) ListLoopNodes(
	ctx context.Context,
	workspaceID string,
	query core.LoopNodeListQuery,
) (contract.LoopNodeInventoryResponse, error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return contract.LoopNodeInventoryResponse{}, err
	}
	reader, ok := s.persistence.(looppkg.NodeInventoryReader)
	if !ok {
		return contract.LoopNodeInventoryResponse{}, errors.New("daemon: loop node inventory is unavailable")
	}
	page, err := reader.ListNodeInventory(ctx, looppkg.NodeInventoryQuery{
		WorkspaceID: ws,
		State:       looppkg.NodeInventoryState(strings.TrimSpace(query.State)),
		LoopName:    query.LoopName,
		RunID:       looppkg.RunID(strings.TrimSpace(query.RunID)),
		Cursor:      query.Cursor,
		Limit:       query.Limit,
	})
	if err != nil {
		return contract.LoopNodeInventoryResponse{}, err
	}
	items := make([]contract.LoopNodeInventoryItem, 0, len(page.Items))
	now := time.Now().UTC()
	if s.now != nil {
		now = s.now().UTC()
	}
	for _, item := range page.Items {
		items = append(items, loopNodeInventoryItemPayload(item, now))
	}
	return contract.LoopNodeInventoryResponse{Items: items, NextCursor: page.NextCursor}, nil
}

func (s *daemonLoopAPIService) loopRunNodeState(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) ([]contract.LoopNodeControlPayload, []contract.LoopNodeWaitPayload, error) {
	controlReader, controlsAvailable := s.persistence.(looppkg.NodeControlReader)
	waitReader, waitsAvailable := s.persistence.(looppkg.NodeWaitReader)
	if !controlsAvailable || !waitsAvailable {
		return []contract.LoopNodeControlPayload{}, []contract.LoopNodeWaitPayload{}, nil
	}
	controls, err := controlReader.ListNodeControls(ctx, workspaceID, runID)
	if err != nil {
		return nil, nil, fmt.Errorf("daemon: list controls for loop run %q: %w", runID, err)
	}
	waits, err := waitReader.ListNodeWaits(ctx, workspaceID, runID)
	if err != nil {
		return nil, nil, fmt.Errorf("daemon: list waits for loop run %q: %w", runID, err)
	}
	controlPayloads := make([]contract.LoopNodeControlPayload, 0, len(controls))
	for _, control := range controls {
		controlPayloads = append(controlPayloads, loopNodeControlPayload(control))
	}
	now := time.Now().UTC()
	if s.now != nil {
		now = s.now().UTC()
	}
	waitPayloads := make([]contract.LoopNodeWaitPayload, 0, len(waits))
	for _, wait := range waits {
		waitPayloads = append(waitPayloads, loopNodeWaitPayload(wait, now))
	}
	return controlPayloads, waitPayloads, nil
}

func (s *daemonLoopAPIService) mutateLoopRunCancellation(
	ctx context.Context,
	workspaceID string,
	runID string,
	kill bool,
	actor taskpkg.ActorContext,
) (contract.LoopMutationResponse, error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return contract.LoopMutationResponse{}, err
	}
	normalizedRunID := looppkg.RunID(strings.TrimSpace(runID))
	if normalizedRunID == "" {
		return contract.LoopMutationResponse{}, fmt.Errorf("%w: run_id is required", looppkg.ErrValidation)
	}
	if kill {
		err = s.aggregate.KillRun(ctx, ws, normalizedRunID, "operator request", actor)
	} else {
		err = s.aggregate.CancelRun(ctx, ws, normalizedRunID, "operator request", actor)
	}
	if err != nil {
		return contract.LoopMutationResponse{}, err
	}
	current, err := s.aggregate.Get(ctx, ws, normalizedRunID)
	if err != nil {
		return contract.LoopMutationResponse{}, err
	}
	return contract.LoopMutationResponse{
		OK: true, RunID: string(normalizedRunID), Status: string(current.Status),
		Provenance: loopRunControlProvenancePayload(current),
	}, nil
}

func (s *daemonLoopAPIService) mutateLoopNodeCancellation(
	ctx context.Context,
	workspaceID string,
	runID string,
	nodeID string,
	reason string,
	kill bool,
	actor taskpkg.ActorContext,
) (contract.LoopMutationResponse, error) {
	ws, normalizedRunID, normalizedNodeID, err := normalizeLoopNodeIdentity(workspaceID, runID, nodeID)
	if err != nil {
		return contract.LoopMutationResponse{}, err
	}
	if kill {
		err = s.aggregate.KillNode(ctx, ws, normalizedRunID, normalizedNodeID, reason, actor)
	} else {
		err = s.aggregate.CancelNode(ctx, ws, normalizedRunID, normalizedNodeID, reason, actor)
	}
	if err != nil {
		return contract.LoopMutationResponse{}, err
	}
	control, err := s.loopNodeControlForMutation(ctx, ws, normalizedRunID, normalizedNodeID)
	if err != nil {
		return contract.LoopMutationResponse{}, err
	}
	return loopNodeMutationResponse(normalizedRunID, normalizedNodeID, control), nil
}

func (s *daemonLoopAPIService) loopNodeControlForMutation(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
	nodeID looppkg.NodeID,
) (*contract.LoopNodeControlPayload, error) {
	reader, ok := s.persistence.(looppkg.NodeControlReader)
	if !ok {
		return nil, errors.New("daemon: loop node control reader is unavailable")
	}
	controls, err := reader.ListNodeControls(ctx, workspaceID, runID)
	if err != nil {
		return nil, fmt.Errorf("daemon: list controls for loop node %q: %w", nodeID, err)
	}
	for _, control := range controls {
		if control.NodeID != nodeID {
			continue
		}
		payload := loopNodeControlPayload(control)
		return &payload, nil
	}
	return nil, nil
}

func (s *daemonLoopAPIService) nodeLifecycleService() (looppkg.NodeLifecycleService, error) {
	lifecycle, ok := s.aggregate.(looppkg.NodeLifecycleService)
	if !ok {
		return nil, errors.New("daemon: loop node lifecycle service is unavailable")
	}
	return lifecycle, nil
}

func normalizeLoopNodeIdentity(
	workspaceID string,
	runID string,
	nodeID string,
) (looppkg.WorkspaceID, looppkg.RunID, looppkg.NodeID, error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return "", "", "", err
	}
	normalizedRunID := looppkg.RunID(strings.TrimSpace(runID))
	normalizedNodeID := looppkg.NodeID(strings.TrimSpace(nodeID))
	if normalizedRunID == "" || normalizedNodeID == "" {
		return "", "", "", fmt.Errorf("%w: run_id and node_id are required", looppkg.ErrValidation)
	}
	return ws, normalizedRunID, normalizedNodeID, nil
}

func loopNodeMutationResponse(
	runID looppkg.RunID,
	nodeID looppkg.NodeID,
	control *contract.LoopNodeControlPayload,
) contract.LoopMutationResponse {
	return contract.LoopMutationResponse{
		OK: true, RunID: string(runID), NodeID: string(nodeID), Control: control,
	}
}

func loopNodeControlPayload(control looppkg.NodeControl) contract.LoopNodeControlPayload {
	return contract.LoopNodeControlPayload{
		LoopRunID:               string(control.LoopRunID),
		NodeID:                  string(control.NodeID),
		Paused:                  control.Paused,
		PauseProvenance:         loopControlProvenancePayload(control.PauseProvenance),
		Quarantined:             control.Quarantined,
		QuarantineEntry:         append(json.RawMessage(nil), control.QuarantineEntry...),
		QuarantinedAt:           cloneTimePointer(control.QuarantinedAt),
		AttentionFlag:           control.AttentionFlag,
		AttentionReason:         control.AttentionReason,
		AttentionProducerNodeID: control.AttentionProducerNodeID,
		CancelState:             string(control.CancelState),
		CancelProvenance:        loopControlProvenancePayload(control.CancelProvenance),
		LastEvidenceAt:          cloneTimePointer(control.LastEvidenceAt),
		DeathResumeStreak:       control.DeathResumeStreak,
		Revision:                control.Revision,
		UpdatedAt:               control.UpdatedAt.UTC(),
	}
}

func loopControlProvenancePayload(
	provenance *looppkg.ControlProvenance,
) *contract.LoopControlProvenancePayload {
	if provenance == nil {
		return nil
	}
	return &contract.LoopControlProvenancePayload{
		ActorKind:   provenance.ActorKind,
		ActorID:     provenance.ActorID,
		Reason:      provenance.Reason,
		RuleID:      provenance.RuleID,
		RequestedAt: provenance.RequestedAt.UTC(),
	}
}

func loopRunControlProvenancePayload(run *looppkg.Run) *contract.LoopControlProvenancePayload {
	if run == nil || (strings.TrimSpace(run.ControlActor.Ref) == "" && run.ControlRequestedAt.IsZero()) {
		return nil
	}
	return &contract.LoopControlProvenancePayload{
		ActorKind:   string(run.ControlActor.Kind),
		ActorID:     run.ControlActor.Ref,
		RequestedAt: run.ControlRequestedAt.UTC(),
	}
}

func loopNodeWaitPayload(wait looppkg.NodeWait, now time.Time) contract.LoopNodeWaitPayload {
	age := max(now.UTC().Sub(wait.CreatedAt.UTC()), 0)
	return contract.LoopNodeWaitPayload{
		LoopRunID:         string(wait.LoopRunID),
		Generation:        wait.Generation,
		NodeID:            string(wait.NodeID),
		ItemIndex:         wait.ItemIndex,
		Kind:              wait.Kind,
		ResumeAt:          cloneTimePointer(wait.ResumeAt),
		NextEscalationAt:  cloneTimePointer(wait.NextEscalationAt),
		EscalationCursor:  wait.EscalationCursor,
		ClaimState:        string(wait.ClaimState),
		ClaimedByKind:     wait.ClaimedByKind,
		ClaimedByID:       wait.ClaimedByID,
		ClaimedAt:         cloneTimePointer(wait.ClaimedAt),
		AdmissionFailures: wait.AdmissionFailures,
		Expect:            append(json.RawMessage(nil), wait.Expect...),
		IssuedEpoch:       wait.IssuedEpoch,
		CreatedAt:         wait.CreatedAt.UTC(),
		AgeSeconds:        int64(age / time.Second),
	}
}

func loopNodeInventoryItemPayload(
	item looppkg.NodeInventoryItem,
	now time.Time,
) contract.LoopNodeInventoryItem {
	payload := contract.LoopNodeInventoryItem{
		State:      contract.LoopNodeInventoryState(item.State),
		LoopRunID:  string(item.LoopRunID),
		LoopName:   item.LoopName,
		Generation: item.Generation,
		NodeID:     string(item.NodeID),
		ItemIndex:  item.ItemIndex,
		StateAt:    item.StateAt.UTC(),
	}
	if item.Wait != nil {
		wait := loopNodeWaitPayload(*item.Wait, now)
		payload.Wait = &wait
	}
	if item.Control != nil {
		control := loopNodeControlPayload(*item.Control)
		payload.Control = &control
	}
	if item.Output != nil {
		attempts := []looppkg.NodeAttempt{}
		if item.Attempt != nil {
			attempts = append(attempts, *item.Attempt)
		}
		outputs := loopGenerationOutputsPayload([]looppkg.GenerationOutput{*item.Output}, attempts)
		payload.Output = &outputs[0]
	}
	return payload
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}
