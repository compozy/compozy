package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// taskRuntimeSessionViews is the session telemetry surface task reads consume.
type taskRuntimeSessionViews interface {
	Status(ctx context.Context, id string) (*session.Info, error)
	Events(ctx context.Context, id string, query store.EventQuery) ([]store.SessionEvent, error)
	TokenUsage(ctx context.Context, id string) ([]store.TokenUsage, error)
}

// taskRuntimeViewReader adapts session-manager telemetry into task run-detail
// reads; sessions boot after the task manager, so it resolves lazily.
type taskRuntimeViewReader struct {
	state *bootState
}

var _ taskpkg.RuntimeViewReader = (*taskRuntimeViewReader)(nil)

func (r *taskRuntimeViewReader) manager() (taskRuntimeSessionViews, error) {
	if r == nil || r.state == nil || r.state.sessions == nil {
		return nil, errors.New("daemon: session manager is unavailable for task runtime views")
	}
	views, ok := r.state.sessions.(taskRuntimeSessionViews)
	if !ok {
		return nil, errors.New("daemon: session manager does not expose task runtime views")
	}
	return views, nil
}

func (r *taskRuntimeViewReader) GetSession(
	ctx context.Context,
	sessionID string,
) (*taskpkg.RunSessionRef, error) {
	sessions, err := r.manager()
	if err != nil {
		return nil, err
	}
	info, err := sessions.Status(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, fmt.Errorf("daemon: session %q status is empty", sessionID)
	}
	return &taskpkg.RunSessionRef{
		SessionID:   info.ID,
		WorkspaceID: info.WorkspaceID,
		AgentName:   info.AgentName,
		Name:        info.Name,
		State:       string(info.State),
		CreatedAt:   info.CreatedAt,
		UpdatedAt:   info.UpdatedAt,
	}, nil
}

func (r *taskRuntimeViewReader) ListSessionEvents(
	ctx context.Context,
	sessionID string,
	query store.EventQuery,
) ([]store.SessionEvent, error) {
	sessions, err := r.manager()
	if err != nil {
		return nil, err
	}
	return sessions.Events(ctx, strings.TrimSpace(sessionID), query)
}

func (r *taskRuntimeViewReader) ListSessionTokenStats(
	ctx context.Context,
	sessionID string,
) ([]store.TokenStats, error) {
	sessions, err := r.manager()
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(sessionID)
	usage, err := sessions.TokenUsage(ctx, trimmed)
	if err != nil {
		return nil, err
	}
	return sessionTokenStatsFromUsage(trimmed, usage), nil
}

// sessionTokenStatsFromUsage projects per-turn usage rows into per-turn stats;
// summaries downstream own aggregation and cost-compatibility rules.
func sessionTokenStatsFromUsage(sessionID string, usage []store.TokenUsage) []store.TokenStats {
	stats := make([]store.TokenStats, 0, len(usage))
	for _, turn := range usage {
		stat := store.TokenStats{
			ID:           turn.TurnID,
			SessionID:    sessionID,
			InputTokens:  turn.InputTokens,
			OutputTokens: turn.OutputTokens,
			TotalTokens:  turn.TotalTokens,
			TotalCost:    turn.CostAmount,
			CostCurrency: turn.CostCurrency,
			CostStatus:   store.TokenCostStatusUnknown,
			CostSource:   store.TokenCostSourceNone,
			TurnCount:    1,
			UpdatedAt:    turn.Timestamp,
		}
		if turn.CostAmount != nil {
			stat.CostStatus = store.TokenCostStatusActual
			stat.CostSource = store.TokenCostSourceAgentReported
		}
		stats = append(stats, stat)
	}
	return stats
}

// SessionTokenTotal sums reported total tokens for one session's usage projection.
func sessionTokenUsageTotal(usage []store.TokenUsage) int64 {
	var total int64
	for _, turn := range usage {
		switch {
		case turn.TotalTokens != nil:
			total += *turn.TotalTokens
		case turn.InputTokens != nil || turn.OutputTokens != nil:
			if turn.InputTokens != nil {
				total += *turn.InputTokens
			}
			if turn.OutputTokens != nil {
				total += *turn.OutputTokens
			}
		}
	}
	return total
}
