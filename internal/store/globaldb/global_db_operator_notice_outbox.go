package globaldb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/compozy/compozy/internal/notifications"
	"github.com/compozy/compozy/internal/observe/attention"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (r *NotificationRepo) EnqueueOperatorNotice(ctx context.Context, notice notifications.OperatorNotice) error {
	if err := r.checkReady(ctx, "enqueue operator notice"); err != nil {
		return err
	}
	return r.executeWriteQuery(
		ctx,
		"enqueue operator notice",
		func(ctx context.Context, queries *sqlcgen.Queries) error {
			return queries.InsertOperatorNotice(ctx, sqlcgen.InsertOperatorNoticeParams{
				ID: notice.NotificationID, ProfileID: notice.ProfileID,
				WorkspaceID: sql.NullString{String: notice.WorkspaceID, Valid: notice.WorkspaceID != ""},
				SessionID:   notice.SessionID, Kind: notice.Kind, Action: notice.Action, Resource: notice.Resource,
				Title: notice.Title, Body: notice.Body, CreatedAt: store.FormatTimestamp(notice.At),
			})
		},
	)
}

func (r *NotificationRepo) ListUnreadOperatorNotices(
	ctx context.Context, scope attention.Scope,
) ([]notifications.OperatorNotice, error) {
	if err := r.checkReady(ctx, "list unread operator notices"); err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	rows, err := r.readQueries.ListUnreadOperatorNotices(ctx, sqlcgen.ListUnreadOperatorNoticesParams{
		ProfileID: scope.ProfileID, ActorKind: scope.ActorKind, ActorID: scope.ActorID,
	})
	if err != nil {
		return nil, fmt.Errorf("store: list unread operator notices: %w", err)
	}
	notices := make([]notifications.OperatorNotice, 0, len(rows))
	for _, row := range rows {
		at, err := store.ParseTimestamp(row.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("store: decode operator notice timestamp: %w", err)
		}
		notices = append(notices, notifications.OperatorNotice{
			NotificationID: row.ID, ProfileID: row.ProfileID, WorkspaceID: row.WorkspaceID.String,
			SessionID: row.SessionID, Kind: row.Kind, Action: row.Action, Resource: row.Resource,
			Title: row.Title, Body: row.Body, At: at,
		})
	}
	return notices, nil
}
