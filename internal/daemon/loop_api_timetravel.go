package daemon

import (
	"context"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/task"
)

func (s *daemonLoopAPIService) DiffLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	query looppkg.DiffQuery,
) (contract.LoopDiffResponse, error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return contract.LoopDiffResponse{}, err
	}
	query.RunID = looppkg.RunID(strings.TrimSpace(runID))
	service, ok := s.aggregate.(looppkg.TimeTravelService)
	if !ok {
		return contract.LoopDiffResponse{}, looppkg.ErrActionDependencyMissing
	}
	result, err := service.DiffRun(ctx, ws, query)
	if err != nil {
		return contract.LoopDiffResponse{}, err
	}
	var response contract.LoopDiffResponse
	if err := transcodeLoopAPI(result, &response); err != nil {
		return contract.LoopDiffResponse{}, err
	}
	return response, nil
}

func (s *daemonLoopAPIService) RerunLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	request contract.RerunLoopRequest,
	actor task.ActorContext,
) (contract.RerunLoopResponse, error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return contract.RerunLoopResponse{}, err
	}
	service, ok := s.aggregate.(looppkg.TimeTravelService)
	if !ok {
		return contract.RerunLoopResponse{}, looppkg.ErrActionDependencyMissing
	}
	result, err := service.RerunFromNode(ctx, looppkg.RerunInput{
		WorkspaceID: ws, RunID: looppkg.RunID(strings.TrimSpace(runID)),
		FromNode: looppkg.NodeID(strings.TrimSpace(request.FromNode)), ItemIndex: request.ItemIndex,
		Reason: strings.TrimSpace(request.Reason), RequestID: strings.TrimSpace(request.RequestID), Actor: actor,
	})
	if err != nil {
		return contract.RerunLoopResponse{}, err
	}
	return contract.RerunLoopResponse{
		RunID: string(result.RunID), Generation: result.Generation,
		ParentGeneration: result.ParentGeneration, RerunNodes: result.RerunNodes,
		Carried: result.Carried, Replayed: result.Replayed,
	}, nil
}

func (s *daemonLoopAPIService) ForkLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	request contract.ForkLoopRequest,
	actor task.ActorContext,
) (contract.ForkLoopResponse, error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return contract.ForkLoopResponse{}, err
	}
	service, ok := s.aggregate.(looppkg.TimeTravelService)
	if !ok {
		return contract.ForkLoopResponse{}, looppkg.ErrActionDependencyMissing
	}
	result, err := service.ForkRun(ctx, looppkg.ForkInput{
		WorkspaceID: ws, RunID: looppkg.RunID(strings.TrimSpace(runID)), Generation: request.Generation,
		Inputs: request.Inputs, Reason: strings.TrimSpace(request.Reason),
		RequestID: strings.TrimSpace(request.RequestID), Actor: actor,
	})
	if err != nil {
		return contract.ForkLoopResponse{}, err
	}
	payload, err := loopRunPayload(result.Run)
	if err != nil {
		return contract.ForkLoopResponse{}, err
	}
	return contract.ForkLoopResponse{Run: payload, Replayed: result.Replayed}, nil
}
