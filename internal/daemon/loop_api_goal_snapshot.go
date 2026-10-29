package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/workspaceaccess"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	goalpkg "github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/session"
)

// GetSessionGoal returns the newest visible session-origin Goal projection.
func (s *daemonLoopAPIService) GetSessionGoal(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (*session.GoalSnapshot, error) {
	ws, projection, visible, err := s.visibleSessionGoalProjection(ctx, workspaceID, sessionID)
	if err != nil || !visible {
		return nil, err
	}
	params, nodeID, contractSummary, err := s.goalDefinitionProjection(ctx, ws, projection)
	if err != nil {
		return nil, fmt.Errorf("daemon: hydrate executed Goal definition %q: %w", projection.DefinitionDigest, err)
	}
	contextSnapshot, err := s.goalContextSnapshot(ctx, ws, projection)
	if err != nil {
		s.logGoalCommandWarning(ctx, "Goal context telemetry is unavailable", workspaceID, sessionID, err)
		contextSnapshot = session.GoalContextSnapshot{
			State: loopManagedContextStateUnknown, NudgeRatio: projection.ContextNudgeRatio,
			Diagnostic: "goal_context_unavailable",
		}
	}
	return composeSessionGoalSnapshot(projection, params, nodeID, contractSummary, contextSnapshot), nil
}

// sessionGoalVisible reports whether GetSessionGoal would find a Goal without hydrating its
// executed definition or context snapshot; tool availability needs only that answer.
func (s *daemonLoopAPIService) sessionGoalVisible(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (bool, error) {
	_, _, visible, err := s.visibleSessionGoalProjection(ctx, workspaceID, sessionID)
	return visible, err
}

func (s *daemonLoopAPIService) visibleSessionGoalProjection(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (looppkg.WorkspaceID, goalpkg.SessionProjection, bool, error) {
	if s == nil || s.goalPersistence == nil {
		return "", goalpkg.SessionProjection{}, false, errors.New("daemon: Goal read persistence is unavailable")
	}
	ref := workspaceID
	if strings.TrimSpace(ref) == "" {
		ref = workspaceaccess.UserSelector
	}
	ws, err := normalizeLoopWorkspaceID(ref)
	if err != nil {
		return "", goalpkg.SessionProjection{}, false, fmt.Errorf(
			"daemon: normalize Goal workspace %q: %w", workspaceID, err,
		)
	}
	targetSessionID := strings.TrimSpace(sessionID)
	if targetSessionID == "" {
		return "", goalpkg.SessionProjection{}, false, fmt.Errorf(
			"%w: Goal session_id is required", looppkg.ErrValidation,
		)
	}
	if s.sessionStatus == nil {
		return "", goalpkg.SessionProjection{}, false, errors.New("daemon: session status reader is unavailable")
	}
	info, err := s.sessionStatus.Status(ctx, targetSessionID)
	if err != nil {
		return "", goalpkg.SessionProjection{}, false, fmt.Errorf(
			"daemon: read Goal session status %q: %w", targetSessionID, err,
		)
	}
	if info == nil || strings.TrimSpace(info.WorkspaceID) != string(ws) {
		return "", goalpkg.SessionProjection{}, false, session.ErrSessionNotFound
	}
	projection, err := s.goalPersistence.GetSessionGoalProjection(ctx, ws, targetSessionID)
	if err != nil {
		return "", goalpkg.SessionProjection{}, false, fmt.Errorf(
			"daemon: read Goal projection for session %q: %w", targetSessionID, err,
		)
	}
	return ws, projection, projection.Found && !projection.Cleared, nil
}

func (s *daemonLoopAPIService) goalDefinitionProjection(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	projection goalpkg.SessionProjection,
) (dsl.GoalParams, string, string, error) {
	run, err := s.persistence.GetLoopRun(ctx, workspaceID, projection.RunID)
	if err != nil {
		return dsl.GoalParams{}, "", "", err
	}
	snapshot, err := s.persistence.GetLoopDefinitionSnapshot(
		ctx,
		workspaceID,
		run.ProfileID,
		projection.DefinitionDigest,
	)
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
