package daemon

import (
	"context"
	"errors"
	"fmt"

	looppkg "github.com/compozy/agh/internal/loop"
	goalpkg "github.com/compozy/agh/internal/loop/goal"
)

func (s *daemonLoopAPIService) findGoalReportTarget(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	sessionID string,
) (goalpkg.ToolReportTarget, bool, error) {
	store, err := s.goalToolStore()
	if err != nil {
		return goalpkg.ToolReportTarget{}, false, fmt.Errorf("daemon: acquire Goal report target store: %w", err)
	}
	target, found, err := store.FindGoalReportTarget(ctx, workspaceID, sessionID)
	if err != nil {
		return goalpkg.ToolReportTarget{}, false, fmt.Errorf(
			"daemon: find Goal report target for session %q: %w",
			sessionID,
			err,
		)
	}
	return target, found, nil
}

func (s *daemonLoopAPIService) resolveActiveGoalOriginAlias(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	sessionID string,
) (string, bool, error) {
	store, err := s.goalToolStore()
	if err != nil {
		return "", false, fmt.Errorf("daemon: acquire Goal origin alias store: %w", err)
	}
	alias, found, err := store.ResolveActiveGoalOriginAlias(ctx, workspaceID, sessionID)
	if err != nil {
		return "", false, fmt.Errorf("daemon: resolve Goal origin alias for session %q: %w", sessionID, err)
	}
	return alias, found, nil
}

func (s *daemonLoopAPIService) recordGoalReport(
	ctx context.Context,
	request goalpkg.RecordToolReportRequest,
) (goalpkg.ReportIntent, error) {
	store, err := s.goalToolStore()
	if err != nil {
		return goalpkg.ReportIntent{}, fmt.Errorf("daemon: acquire Goal report store: %w", err)
	}
	report, err := store.RecordGoalReport(ctx, request)
	if err != nil {
		return goalpkg.ReportIntent{}, fmt.Errorf(
			"daemon: record Goal report for prompt %q: %w",
			request.Target.PromptID,
			err,
		)
	}
	return report, nil
}

func (s *daemonLoopAPIService) goalToolStore() (goalpkg.ToolStore, error) {
	if s == nil || s.goalPersistence == nil {
		return nil, errors.New("daemon: Goal tool persistence is unavailable")
	}
	store, ok := s.goalPersistence.(goalpkg.ToolStore)
	if !ok {
		return nil, errors.New("daemon: Goal tool persistence is unavailable")
	}
	return store, nil
}
