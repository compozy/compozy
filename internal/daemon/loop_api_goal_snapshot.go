package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	goalpkg "github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/session"
)

// GetSessionGoal returns the newest visible session-origin Goal projection.
func (s *daemonLoopAPIService) GetSessionGoal(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (*session.GoalSnapshot, error) {
	if s == nil || s.goalPersistence == nil {
		return nil, errors.New("daemon: Goal read persistence is unavailable")
	}
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("daemon: normalize Goal workspace %q: %w", workspaceID, err)
	}
	targetSessionID := strings.TrimSpace(sessionID)
	if targetSessionID == "" {
		return nil, fmt.Errorf("%w: Goal session_id is required", looppkg.ErrValidation)
	}
	if s.sessionStatus == nil {
		return nil, errors.New("daemon: session status reader is unavailable")
	}
	info, err := s.sessionStatus.Status(ctx, targetSessionID)
	if err != nil {
		return nil, fmt.Errorf("daemon: read Goal session status %q: %w", targetSessionID, err)
	}
	if info == nil || strings.TrimSpace(info.WorkspaceID) != string(ws) {
		return nil, session.ErrSessionNotFound
	}
	projection, err := s.goalPersistence.GetSessionGoalProjection(ctx, ws, targetSessionID)
	if err != nil {
		return nil, fmt.Errorf("daemon: read Goal projection for session %q: %w", targetSessionID, err)
	}
	if !projection.Found || projection.Cleared {
		return nil, nil
	}
	params, nodeID, contractSummary, err := s.goalDefinitionProjection(ctx, ws, projection)
	if err != nil {
		return nil, fmt.Errorf("daemon: hydrate executed Goal definition %q: %w", projection.DefinitionDigest, err)
	}
	contextSnapshot, err := s.goalContextSnapshot(ctx, ws, projection)
	if err != nil {
		return nil, fmt.Errorf("daemon: build Goal context snapshot for run %q: %w", projection.RunID, err)
	}
	return composeSessionGoalSnapshot(projection, params, nodeID, contractSummary, contextSnapshot), nil
}

func (s *daemonLoopAPIService) goalDefinitionProjection(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	projection goalpkg.SessionProjection,
) (dsl.GoalParams, string, string, error) {
	snapshot, err := s.persistence.GetLoopDefinitionSnapshot(ctx, workspaceID, projection.DefinitionDigest)
	if err != nil {
		return dsl.GoalParams{}, "", "", fmt.Errorf(
			"daemon: load Goal definition snapshot %q: %w",
			projection.DefinitionDigest,
			err,
		)
	}
	resolved, err := looppkg.LoadExecutedDefinitionSnapshot(snapshot.Definition, projection.DefinitionDigest)
	if err != nil {
		return dsl.GoalParams{}, "", "", fmt.Errorf(
			"daemon: load executed Goal definition %q: %w",
			projection.DefinitionDigest,
			err,
		)
	}
	wantedNodeID := ""
	if projection.Checkpoint != nil {
		wantedNodeID = strings.TrimSpace(string(projection.Checkpoint.Key.NodeID))
	}
	for _, node := range resolved.Definition.Graph.Nodes {
		if node.Kind != loopManagedGoalKind || (wantedNodeID != "" && string(node.ID) != wantedNodeID) {
			continue
		}
		var params dsl.GoalParams
		if err := node.Params.Decode(&params); err != nil {
			return dsl.GoalParams{}, "", "", fmt.Errorf("daemon: decode Goal params: %w", err)
		}
		return params, string(node.ID), goalContractSummary(resolved.Definition, params), nil
	}
	return dsl.GoalParams{}, "", "", fmt.Errorf(
		"%w: Goal node %q is absent from the executed definition",
		looppkg.ErrValidation,
		wantedNodeID,
	)
}

func goalContractSummary(definition dsl.Definition, params dsl.GoalParams) string {
	if summary := strings.TrimSpace(definition.Contract.DefinitionOfDone); summary != "" {
		return summary
	}
	parts := make([]string, 0, len(params.Judge))
	for _, criterion := range params.Judge {
		for _, candidate := range []string{criterion.Rubric, criterion.Expect, criterion.Check, criterion.ID} {
			if value := strings.TrimSpace(candidate); value != "" {
				parts = append(parts, value)
				break
			}
		}
	}
	return strings.Join(parts, "; ")
}
