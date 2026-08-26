package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (s *daemonLoopAPIService) GetLoopConfig(
	ctx context.Context,
	workspaceID string,
	profileID string,
	name string,
) (contract.LoopConfigResponse, error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return contract.LoopConfigResponse{}, err
	}
	snapshot, err := s.aggregate.GetConfigSnapshot(ctx, ws, profileID, name)
	if err != nil {
		return contract.LoopConfigResponse{}, err
	}
	payload, err := loopConfigPayload(snapshot.Stored)
	if err != nil {
		return contract.LoopConfigResponse{}, err
	}
	effective, err := loopEffectiveConfigPayload(snapshot.Effective)
	if err != nil {
		return contract.LoopConfigResponse{}, err
	}
	return contract.LoopConfigResponse{Config: payload, EffectiveConfig: effective}, nil
}

func (s *daemonLoopAPIService) PutLoopConfig(
	ctx context.Context,
	workspaceID string,
	profileID string,
	name string,
	req contract.PutLoopConfigRequest,
) (contract.LoopConfigResponse, error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return contract.LoopConfigResponse{}, err
	}
	cfg, err := loopConfigDomain(req.Config)
	if err != nil {
		return contract.LoopConfigResponse{}, err
	}
	if err := s.aggregate.Configure(ctx, ws, profileID, name, cfg); err != nil {
		return contract.LoopConfigResponse{}, err
	}
	snapshot, err := s.aggregate.GetConfigSnapshot(ctx, ws, profileID, name)
	if err != nil {
		return contract.LoopConfigResponse{}, err
	}
	payload, err := loopConfigPayload(snapshot.Stored)
	if err != nil {
		return contract.LoopConfigResponse{}, err
	}
	effective, err := loopEffectiveConfigPayload(snapshot.Effective)
	if err != nil {
		return contract.LoopConfigResponse{}, err
	}
	return contract.LoopConfigResponse{Config: payload, EffectiveConfig: effective}, nil
}

func (s *daemonLoopAPIService) GetLoopAnnotations(
	ctx context.Context,
	workspaceID string,
	profileID string,
	name string,
) (contract.LoopAnnotationsResponse, error) {
	ws, _, err := s.findLoopRecord(ctx, workspaceID, profileID, name)
	if err != nil {
		return contract.LoopAnnotationsResponse{}, err
	}
	annotations, err := s.persistence.ListLoopUIAnnotations(ctx, ws, name)
	if err != nil {
		return contract.LoopAnnotationsResponse{}, err
	}
	return contract.LoopAnnotationsResponse{Annotations: loopAnnotationsPayload(annotations)}, nil
}

func (s *daemonLoopAPIService) PutLoopAnnotations(
	ctx context.Context,
	workspaceID string,
	profileID string,
	name string,
	req contract.PutLoopAnnotationsRequest,
) (contract.LoopAnnotationsResponse, error) {
	ws, _, err := s.findLoopRecord(ctx, workspaceID, profileID, name)
	if err != nil {
		return contract.LoopAnnotationsResponse{}, err
	}
	annotations := loopAnnotationsDomain(req.Annotations)
	if err := s.persistence.ReplaceLoopUIAnnotations(ctx, ws, name, annotations); err != nil {
		return contract.LoopAnnotationsResponse{}, err
	}
	return contract.LoopAnnotationsResponse{Annotations: loopAnnotationsPayload(annotations)}, nil
}

func (s *daemonLoopAPIService) GetLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
) (contract.LoopRunResponse, error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return contract.LoopRunResponse{}, err
	}
	run, err := s.aggregate.Get(ctx, ws, looppkg.RunID(strings.TrimSpace(runID)))
	if err != nil {
		return contract.LoopRunResponse{}, err
	}
	generations, err := s.loopGenerations(ctx, *run)
	if err != nil {
		return contract.LoopRunResponse{}, err
	}
	executedDefinition, materializedContract, effective, err := s.loadExecutedLoopDefinition(ctx, ws, run)
	if err != nil {
		return contract.LoopRunResponse{}, err
	}
	effectivePayload, err := loopEffectiveConfigPayload(effective)
	if err != nil {
		return contract.LoopRunResponse{}, err
	}
	payload, err := loopRunPayload(*run)
	if err != nil {
		return contract.LoopRunResponse{}, err
	}
	watchEvents, err := loopWatchEventsReadModel(*run, &executedDefinition, generations)
	if err != nil {
		return contract.LoopRunResponse{}, err
	}
	controls, waits, err := s.loopRunNodeState(ctx, ws, run.ID)
	if err != nil {
		return contract.LoopRunResponse{}, err
	}
	requests, err := s.loopRunRequests(ctx, ws, run.ID)
	if err != nil {
		return contract.LoopRunResponse{}, err
	}
	amendments, err := s.loopRunAmendments(ctx, ws, run.ID)
	if err != nil {
		return contract.LoopRunResponse{}, err
	}
	return contract.LoopRunResponse{
		Run:                  payload,
		ExecutedDefinition:   &executedDefinition,
		MaterializedContract: materializedContract,
		EffectiveConfig:      effectivePayload,
		Generations:          generations,
		NodeControls:         controls,
		Waits:                waits,
		Requests:             requests,
		Amendments:           amendments,
		WatchEvents:          watchEvents,
	}, nil
}

func (s *daemonLoopAPIService) loadExecutedLoopDefinition(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	run *looppkg.Run,
) (contract.LoopDefinitionDocument, contract.LoopContract, looppkg.EffectiveConfig, error) {
	snapshot, err := s.persistence.GetLoopDefinitionSnapshot(ctx, workspaceID, run.DefinitionDigest)
	if err != nil {
		return contract.LoopDefinitionDocument{}, contract.LoopContract{}, looppkg.EffectiveConfig{}, fmt.Errorf(
			"daemon: load executed definition snapshot %q for run %q: %w",
			run.DefinitionDigest,
			run.ID,
			err,
		)
	}
	resolved, err := looppkg.LoadExecutedDefinitionSnapshot(snapshot.Definition, run.DefinitionDigest)
	if err != nil {
		return contract.LoopDefinitionDocument{}, contract.LoopContract{}, looppkg.EffectiveConfig{}, fmt.Errorf(
			"daemon: hydrate executed definition %q for run %q: %w",
			run.DefinitionDigest,
			run.ID,
			err,
		)
	}
	executedDefinitionJSON, err := json.Marshal(resolved.Definition)
	if err != nil {
		return contract.LoopDefinitionDocument{}, contract.LoopContract{}, looppkg.EffectiveConfig{}, fmt.Errorf(
			"daemon: marshal executed Loop definition: %w",
			err,
		)
	}
	executedDefinition, err := loopDefinitionDocumentFromJSON(executedDefinitionJSON)
	if err != nil {
		return contract.LoopDefinitionDocument{}, contract.LoopContract{}, looppkg.EffectiveConfig{}, err
	}
	materialized, err := looppkg.MaterializeContract(resolved.Definition.Contract, run.Inputs)
	if err != nil {
		return contract.LoopDefinitionDocument{}, contract.LoopContract{}, looppkg.EffectiveConfig{}, fmt.Errorf(
			"daemon: materialize executed Loop contract for run %q: %w",
			run.ID,
			err,
		)
	}
	var materializedContract contract.LoopContract
	if err := transcodeLoopAPI(materialized, &materializedContract); err != nil {
		return contract.LoopDefinitionDocument{}, contract.LoopContract{}, looppkg.EffectiveConfig{}, fmt.Errorf(
			"daemon: encode materialized Loop contract: %w",
			err,
		)
	}
	return executedDefinition, materializedContract, resolved.EffectiveConfig, nil
}

func (s *daemonLoopAPIService) loopRunAmendments(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) ([]contract.LoopNodeAmendmentPayload, error) {
	store, ok := s.persistence.(looppkg.AmendmentStore)
	if !ok {
		return []contract.LoopNodeAmendmentPayload{}, nil
	}
	amendments, err := store.ListNodeAmendments(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	payloads := make([]contract.LoopNodeAmendmentPayload, 0, len(amendments))
	for _, amendment := range amendments {
		payload, err := loopNodeAmendmentPayload(amendment)
		if err != nil {
			return nil, err
		}
		payloads = append(payloads, payload)
	}
	return payloads, nil
}

const loopRequestStateResolved = "resolved"

func (s *daemonLoopAPIService) loopRunRequests(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) ([]contract.LoopRequestPayload, error) {
	requests := make([]contract.LoopRequestPayload, 0)
	for _, state := range []string{looppkg.RequestStatePending, loopRequestStateResolved} {
		cursor := ""
		for {
			page, err := s.aggregate.ListRequests(ctx, workspaceID, looppkg.RequestQuery{
				RunID: runID, State: state, Limit: 200, Cursor: cursor,
			})
			if err != nil {
				return nil, err
			}
			for _, request := range page.Items {
				requests = append(requests, loopRequestPayload(request))
			}
			if page.NextCursor == "" {
				break
			}
			cursor = page.NextCursor
		}
	}
	return requests, nil
}

func (s *daemonLoopAPIService) PauseLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	actor taskpkg.ActorContext,
) error {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return err
	}
	return s.aggregate.Pause(ctx, ws, looppkg.RunID(strings.TrimSpace(runID)), actor)
}

func (s *daemonLoopAPIService) ResumeLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	actor taskpkg.ActorContext,
) error {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return err
	}
	return s.aggregate.Resume(ctx, ws, looppkg.RunID(strings.TrimSpace(runID)), actor)
}

func (s *daemonLoopAPIService) ApproveLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	req contract.ApproveLoopRunRequest,
	actor taskpkg.ActorContext,
) error {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return err
	}
	normalizedRunID := looppkg.RunID(strings.TrimSpace(runID))
	if s.responderPolicy != nil {
		denied, policyErr := s.responderPolicy.DeniesSelfOperation(
			ctx, string(ws), string(normalizedRunID), actor,
		)
		if policyErr != nil {
			return policyErr
		}
		if denied {
			return fmt.Errorf(
				"%w: loop run %q cannot be approved by its initiator chain",
				taskpkg.ErrPermissionDenied,
				normalizedRunID,
			)
		}
	} else if run, getErr := s.aggregate.Get(ctx, ws, normalizedRunID); getErr != nil {
		return getErr
	} else if loopApprovalSelfDenied(*run, actor) {
		return fmt.Errorf(
			"%w: loop run %q cannot be approved by its starter session",
			taskpkg.ErrPermissionDenied,
			normalizedRunID,
		)
	}
	return s.aggregate.Approve(
		ctx,
		ws,
		normalizedRunID,
		looppkg.NodeID(strings.TrimSpace(req.GateID)),
		looppkg.GateDecision(req.Decision),
		actor,
	)
}

func loopApprovalSelfDenied(run looppkg.Run, actor taskpkg.ActorContext) bool {
	return actor.Actor.Kind.Normalize() == taskpkg.ActorKindAgentSession &&
		run.StartedBy.Kind.Normalize() == taskpkg.ActorKindAgentSession &&
		strings.TrimSpace(actor.Actor.Ref) != "" &&
		strings.TrimSpace(actor.Actor.Ref) == strings.TrimSpace(run.StartedBy.Ref)
}

func (s *daemonLoopAPIService) ListLoopRunEvents(
	ctx context.Context,
	workspaceID string,
	runID string,
	afterSeq int64,
	readScope store.ReadScope,
) ([]contract.LoopRunEventPayload, error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return nil, err
	}
	events, err := s.persistence.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
		ReadScope:   readScope,
		WorkspaceID: ws,
		RunID:       looppkg.RunID(strings.TrimSpace(runID)),
		AfterSeq:    afterSeq,
	})
	if err != nil {
		return nil, err
	}
	payloads := make([]contract.LoopRunEventPayload, 0, len(events))
	for _, event := range events {
		payloads = append(payloads, loopRunEventPayload(event))
	}
	return payloads, nil
}

func loopPlanPayload(plan *looppkg.PlanPreview) (*contract.LoopPlanPayload, error) {
	if plan == nil {
		return nil, nil
	}
	var loopContract contract.LoopContract
	if err := transcodeLoopAPI(plan.Contract, &loopContract); err != nil {
		return nil, fmt.Errorf("daemon: encode loop plan contract DTO: %w", err)
	}
	var materializedContract contract.LoopContract
	if err := transcodeLoopAPI(plan.MaterializedContract, &materializedContract); err != nil {
		return nil, fmt.Errorf("daemon: encode materialized loop plan contract DTO: %w", err)
	}
	effective, err := loopEffectiveConfigPayload(plan.EffectiveConfig)
	if err != nil {
		return nil, err
	}
	resolvedInputs, err := cloneLoopAPIMap(plan.ResolvedInputs)
	if err != nil {
		return nil, err
	}
	return &contract.LoopPlanPayload{
		LoopName:                     plan.LoopName,
		ResolvedInputs:               resolvedInputs,
		InputOrigins:                 loopInputOriginsPayload(plan.InputOrigins),
		Generation:                   plan.Generation,
		Nodes:                        loopPlanNodesPayload(plan.Nodes),
		Contract:                     loopContract,
		MaterializedContract:         materializedContract,
		EffectiveConfig:              effective,
		ResolvedNetworkParticipation: participation.CloneSpec(plan.ResolvedNetworkParticipation),
	}, nil
}

func loopInputOriginsPayload(input map[string]looppkg.InputOrigin) map[string]contract.LoopInputOrigin {
	if len(input) == 0 {
		return map[string]contract.LoopInputOrigin{}
	}
	origins := make(map[string]contract.LoopInputOrigin, len(input))
	for key, origin := range input {
		origins[key] = contract.LoopInputOrigin(origin)
	}
	return origins
}

func loopAnnotationsPayload(input []looppkg.UIAnnotation) []contract.LoopAnnotationPayload {
	payloads := make([]contract.LoopAnnotationPayload, 0, len(input))
	for _, annotation := range input {
		payloads = append(payloads, contract.LoopAnnotationPayload{
			NodeID: string(annotation.NodeID),
			X:      annotation.X,
			Y:      annotation.Y,
		})
	}
	return payloads
}

func loopAnnotationsDomain(input []contract.LoopAnnotationPayload) []looppkg.UIAnnotation {
	annotations := make([]looppkg.UIAnnotation, 0, len(input))
	for _, annotation := range input {
		annotations = append(annotations, looppkg.UIAnnotation{
			NodeID: looppkg.NodeID(strings.TrimSpace(annotation.NodeID)),
			X:      annotation.X,
			Y:      annotation.Y,
		})
	}
	return annotations
}

func normalizeLoopWorkspaceID(workspaceID string) (looppkg.WorkspaceID, error) {
	trimmed := strings.TrimSpace(workspaceID)
	if trimmed == "" {
		return "", fmt.Errorf("%w: workspace_id is required", looppkg.ErrValidation)
	}
	return looppkg.WorkspaceID(trimmed), nil
}
