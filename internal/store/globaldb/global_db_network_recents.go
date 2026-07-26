package globaldb

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

// ListNetworkRecents returns the newest thread/direct summaries across every
// channel in one workspace using the materialized conversation tables.
func (g *NetworkRepo) ListNetworkRecents(
	ctx context.Context,
	query store.NetworkRecentQuery,
) ([]store.NetworkRecentSummary, error) {
	if err := g.checkReady(ctx, "list network recents"); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, fmt.Errorf("store: validate network recent query: %w", err)
	}
	workspaceID := strings.TrimSpace(query.WorkspaceID)
	generatedRows, err := g.queries.ListNetworkRecents(ctx, sqlcgen.ListNetworkRecentsParams{
		WorkspaceID: workspaceID,
		RowLimit:    int64(query.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("store: query network recents: %w", err)
	}

	recents := make([]store.NetworkRecentSummary, 0, len(generatedRows))
	for _, row := range generatedRows {
		activity, parseErr := store.ParseTimestamp(row.LastActivityAt)
		if parseErr != nil {
			return nil, fmt.Errorf("store: parse network recent activity: %w", parseErr)
		}
		recents = append(recents, store.NetworkRecentSummary{
			WorkspaceID:          row.WorkspaceID,
			Channel:              row.Channel,
			Surface:              row.Surface,
			ContainerID:          row.ContainerID,
			LastActivityAt:       activity,
			LastActivitySequence: row.LastActivitySequence,
			LastMessagePreview:   row.LastMessagePreview,
			Title:                row.Title,
			ParticipantCount:     int(row.ParticipantCount),
			SessionA:             row.SessionA,
			SessionB:             row.SessionB,
		})
	}
	return recents, nil
}
