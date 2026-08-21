package daemon

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
)

func (s *daemonLoopAPIService) GetLoopRunNodes(
	ctx context.Context,
	workspaceID string,
	runID string,
	query looppkg.RosterQuery,
) (contract.LoopRunNodesResponse, error) {
	if s == nil || s.runReads == nil {
		return contract.LoopRunNodesResponse{}, errors.New("daemon: loop read service is unavailable")
	}
	return s.runReads.NodeRoster(ctx, workspaceID, looppkg.RunID(runID), query)
}

func (s *daemonLoopAPIService) GetLoopRunBriefing(
	ctx context.Context,
	workspaceID string,
	runID string,
) (contract.LoopBriefingResponse, error) {
	if s == nil || s.runReads == nil {
		return contract.LoopBriefingResponse{}, errors.New("daemon: loop read service is unavailable")
	}
	return s.runReads.Briefing(ctx, workspaceID, looppkg.RunID(runID))
}

func (s *daemonLoopAPIService) GetLoopRunTimeline(
	ctx context.Context,
	workspaceID string,
	runID string,
	query looppkg.TimelineQuery,
) (contract.LoopTimelineResponse, error) {
	if s == nil || s.runReads == nil {
		return contract.LoopTimelineResponse{}, errors.New("daemon: loop read service is unavailable")
	}
	return s.runReads.Timeline(ctx, workspaceID, looppkg.RunID(runID), query)
}
