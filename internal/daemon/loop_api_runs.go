package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (s *daemonLoopAPIService) GetLoopConfig(
	ctx context.Context,
	workspaceID string,
	name string,
) (contract.LoopConfigResponse, error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return contract.LoopConfigResponse{}, err
	}
	snapshot, err := s.aggregate.GetConfigSnapshot(ctx, ws, name)
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
	if err := s.aggregate.Configure(ctx, ws, name, cfg); err != nil {
		return contract.LoopConfigResponse{}, err
	}
	snapshot, err := s.aggregate.GetConfigSnapshot(ctx, ws, name)
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
	name string,
) (contract.LoopAnnotationsResponse, error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
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
	name string,
	req contract.PutLoopAnnotationsRequest,
) (contract.LoopAnnotationsResponse, error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return contract.LoopAnnotationsResponse{}, err
	}
	annotations := loopAnnotationsDomain(req.Annotations)
	if err := s.persistence.ReplaceLoopUIAnnotations(ctx, ws, name, annotations); err != nil {
		return contract.LoopAnnotationsResponse{}, err
	}
	return contract.LoopAnnotationsResponse{Annotations: loopAnnotationsPayload(annotations)}, nil
}

func (s *daemonLoopAPIService) ListLoopRuns(
	ctx context.Context,
	workspaceID string,
	query core.LoopRunListQuery,
) (contract.LoopRunsResponse, error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return contract.LoopRunsResponse{}, err
	}
	runs, err := s.persistence.ListLoopRuns(ctx, looppkg.RunListQuery{
		WorkspaceID:     ws,
		LoopName:        strings.TrimSpace(query.LoopName),
		Status:          looppkg.Status(strings.TrimSpace(query.Status)),
		OriginKind:      strings.TrimSpace(query.Origin),
		OriginSessionID: strings.TrimSpace(query.OriginSession),
		Live:            query.Live,
		Limit:           query.Limit,
	})
	if err != nil {
		return contract.LoopRunsResponse{}, err
	}
	payloads := make([]contract.LoopRunPayload, 0, len(runs))
	for _, run := range runs {
		payload, err := loopRunPayload(run)
		if err != nil {
			return contract.LoopRunsResponse{}, err
		}
		payloads = append(payloads, payload)
	}
	return contract.LoopRunsResponse{
		Runs:       payloads,
		Aggregates: loopRunsAggregate(runs),
	}, nil
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
	snapshot, err := s.persistence.GetLoopDefinitionSnapshot(ctx, ws, run.DefinitionDigest)
	if err != nil {
		return contract.LoopRunResponse{}, fmt.Errorf(
			"daemon: load executed definition snapshot %q for run %q: %w",
			run.DefinitionDigest,
			run.ID,
			err,
		)
	}
	resolved, err := looppkg.LoadExecutedDefinitionSnapshot(snapshot.Definition, run.DefinitionDigest)
	if err != nil {
		return contract.LoopRunResponse{}, fmt.Errorf(
			"daemon: hydrate executed definition %q for run %q: %w",
			run.DefinitionDigest,
			run.ID,
			err,
		)
	}
	executedDefinitionJSON, err := json.Marshal(resolved.Definition)
	if err != nil {
		return contract.LoopRunResponse{}, fmt.Errorf("daemon: marshal executed Loop definition: %w", err)
	}
	executedDefinition, err := loopDefinitionDocumentFromJSON(executedDefinitionJSON)
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
	return contract.LoopRunResponse{
		Run:                payload,
		ExecutedDefinition: &executedDefinition,
		Generations:        generations,
		WatchEvents:        watchEvents,
	}, nil
}

func (s *daemonLoopAPIService) StopLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	actor taskpkg.ActorContext,
) error {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return err
	}
	return s.aggregate.Stop(ctx, ws, looppkg.RunID(strings.TrimSpace(runID)), looppkg.StopReasonOperator, actor)
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
	run, err := s.aggregate.Get(ctx, ws, normalizedRunID)
	if err != nil {
		return err
	}
	if loopApprovalSelfDenied(*run, actor) {
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
) ([]contract.LoopRunEventPayload, error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return nil, err
	}
	events, err := s.persistence.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
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

func (s *daemonLoopAPIService) loopGenerations(
	ctx context.Context,
	run looppkg.Run,
) ([]contract.LoopGenerationPayload, error) {
	if run.Generation <= 0 {
		return nil, nil
	}
	generations := make([]contract.LoopGenerationPayload, 0, run.Generation)
	for generation := 1; generation <= run.Generation; generation++ {
		outputs, err := s.persistence.ListGenerationOutputs(ctx, run.ID, generation)
		if err != nil {
			return nil, err
		}
		generations = append(generations, contract.LoopGenerationPayload{
			Generation: generation,
			Outputs:    loopGenerationOutputsPayload(outputs),
		})
	}
	return generations, nil
}

func loopPlanPayload(plan *looppkg.PlanPreview) (*contract.LoopPlanPayload, error) {
	if plan == nil {
		return nil, nil
	}
	var loopContract contract.LoopContract
	if err := transcodeLoopAPI(plan.Contract, &loopContract); err != nil {
		return nil, fmt.Errorf("daemon: encode loop plan contract DTO: %w", err)
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
		Generation:                   plan.Generation,
		Nodes:                        loopPlanNodesPayload(plan.Nodes),
		Contract:                     loopContract,
		EffectiveConfig:              effective,
		ResolvedNetworkParticipation: participation.CloneSpec(plan.ResolvedNetworkParticipation),
	}, nil
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
