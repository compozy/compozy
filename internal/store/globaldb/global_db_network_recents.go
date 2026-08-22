package globaldb

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// ListNetworkRecents returns the newest thread/direct summaries across every
// channel in one workspace using the materialized conversation tables.
func (g *NetworkRepo) ListNetworkRecents(
	ctx context.Context,
	query store.NetworkRecentQuery,
) (recents []store.NetworkRecentSummary, err error) {
	if err := g.checkReady(ctx, "list network recents"); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, fmt.Errorf("store: validate network recent query: %w", err)
	}
	statement := `SELECT recents.profile_id, profiles.name, profiles.color, COALESCE(profiles.icon, ''),
		COALESCE(profiles.emoji, ''), profiles.archived_at IS NOT NULL,
		recents.workspace_id, recents.channel, recents.surface, recents.container_id,
		recents.last_activity_at, recents.last_activity_sequence, recents.last_message_preview,
		recents.title, recents.participant_count, recents.session_a, recents.session_b
	FROM (
		SELECT profile_id, workspace_id, channel, 'thread' AS surface, thread_id AS container_id,
			last_activity_at, last_activity_sequence, last_message_preview, title, participant_count,
			'' AS session_a, '' AS session_b
		FROM network_threads
		UNION ALL
		SELECT profile_id, workspace_id, channel, 'direct' AS surface, direct_id AS container_id,
			last_activity_at, last_activity_sequence, last_message_preview, '' AS title,
			0 AS participant_count, session_a, session_b
		FROM network_direct_rooms
	) AS recents
	JOIN profiles ON profiles.id = recents.profile_id`
	where, args := store.BuildClauses(
		store.ReadScopeClause("recents.profile_id", query.ReadScope),
		store.StringClause("recents.workspace_id", strings.TrimSpace(query.WorkspaceID)),
	)
	statement = store.AppendWhere(statement, where)
	statement += " ORDER BY recents.last_activity_sequence DESC, recents.surface ASC, recents.container_id ASC"
	statement, args = store.AppendLimit(statement, args, query.Limit)
	rows, err := g.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query network recents: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			closeErr = fmt.Errorf("store: close network recent rows: %w", closeErr)
			if err != nil {
				err = errors.Join(err, closeErr)
				return
			}
			err = closeErr
		}
	}()
	recents = make([]store.NetworkRecentSummary, 0)
	for rows.Next() {
		var row store.NetworkRecentSummary
		var activityRaw string
		if scanErr := rows.Scan(
			&row.ProfileID, &row.ProfileName, &row.ProfileColor, &row.ProfileIcon,
			&row.ProfileEmoji, &row.ProfileArchived, &row.WorkspaceID, &row.Channel,
			&row.Surface, &row.ContainerID, &activityRaw, &row.LastActivitySequence,
			&row.LastMessagePreview, &row.Title, &row.ParticipantCount, &row.SessionA, &row.SessionB,
		); scanErr != nil {
			return nil, fmt.Errorf("store: scan network recent: %w", scanErr)
		}
		activity, parseErr := store.ParseTimestamp(activityRaw)
		if parseErr != nil {
			return nil, fmt.Errorf("store: parse network recent activity: %w", parseErr)
		}
		row.LastActivityAt = activity
		recents = append(recents, row)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("store: iterate network recent rows: %w", rowsErr)
	}
	return recents, nil
}
