package daemon

import (
	"context"
	"errors"
	"strings"

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
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return contract.LoopRunNodesResponse{}, err
	}
	return s.runReads.NodeRoster(ctx, string(ws), looppkg.RunID(strings.TrimSpace(runID)), query)
}

func (s *daemonLoopAPIService) GetLoopRunBriefing(
	ctx context.Context,
	workspaceID string,
	runID string,
) (contract.LoopBriefingResponse, error) {
	if s == nil || s.runReads == nil {
		return contract.LoopBriefingResponse{}, errors.New("daemon: loop read service is unavailable")
	}
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return contract.LoopBriefingResponse{}, err
	}
	return s.runReads.Briefing(ctx, string(ws), looppkg.RunID(strings.TrimSpace(runID)))
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
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return contract.LoopTimelineResponse{}, err
	}
	return s.runReads.Timeline(ctx, string(ws), looppkg.RunID(strings.TrimSpace(runID)), query)
}
