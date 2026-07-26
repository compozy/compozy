package globaldb

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

const maxSessionHealthBatchSize = 100

// ListSessionHealthByIDs loads health for one bounded session catalog page in one query.
func (g *SessionRepo) ListSessionHealthByIDs(
	ctx context.Context,
	sessionIDs []string,
) ([]heartbeat.SessionHealth, error) {
	if err := g.checkReady(ctx, "list session health by ids"); err != nil {
		return nil, err
	}
	ids, _, err := normalizeSessionHealthBatchIDs(sessionIDs)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []heartbeat.SessionHealth{}, nil
	}

	rows, err := g.queries.ListSessionHealthByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("store: query session health: %w", err)
	}
	healthRows := make([]heartbeat.SessionHealth, 0, len(rows))
	for _, row := range rows {
		health, mapErr := sessionHealthFromGenerated(row)
		if mapErr != nil {
			return nil, mapErr
		}
		healthRows = append(healthRows, health)
	}
	return healthRows, nil
}

func sessionHealthFromGenerated(row sqlcgen.SessionHealth) (heartbeat.SessionHealth, error) {
	health := heartbeat.SessionHealth{
		SessionID: row.SessionID, WorkspaceID: row.WorkspaceID, AgentName: row.AgentName,
		State: heartbeat.SessionHealthState(row.State), Health: heartbeat.SessionHealthStatus(row.Health),
		ActivePrompt: row.ActivePrompt, Attachable: row.Attachable, EligibleForWake: row.EligibleForWake,
		IneligibilityReason: heartbeatNullStringValue(row.IneligibilityReason),
		LastError:           heartbeatNullStringValue(row.LastError),
	}
	if err := assignNullableHeartbeatTimestamp(&health.LastActivityAt, row.LastActivityAt); err != nil {
		return heartbeat.SessionHealth{}, fmt.Errorf("store: parse session health last_activity_at: %w", err)
	}
	if err := assignNullableHeartbeatTimestamp(&health.LastPresenceAt, row.LastPresenceAt); err != nil {
		return heartbeat.SessionHealth{}, fmt.Errorf("store: parse session health last_presence_at: %w", err)
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return heartbeat.SessionHealth{}, fmt.Errorf("store: parse session health updated_at: %w", err)
	}
	health.UpdatedAt = updatedAt
	if err := health.Validate(); err != nil {
		return heartbeat.SessionHealth{}, err
	}
	return health.Normalize(), nil
}

func normalizeSessionHealthBatchIDs(input []string) ([]string, []any, error) {
	if len(input) > maxSessionHealthBatchSize {
		return nil, nil, fmt.Errorf(
			"%w: session health batch exceeds maximum %d",
			heartbeat.ErrInvalidSessionHealth,
			maxSessionHealthBatchSize,
		)
	}
	ids := make([]string, 0, len(input))
	args := make([]any, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, raw := range input {
		id := strings.TrimSpace(raw)
		if id == "" {
			return nil, nil, fmt.Errorf("%w: session id is required", heartbeat.ErrInvalidSessionHealth)
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
		args = append(args, id)
	}
	return ids, args, nil
}
