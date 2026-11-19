package core

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/notifications"
	"github.com/compozy/compozy/internal/observe/attention"
)

type operatorNoticeReader interface {
	ListUnreadOperatorNotices(context.Context, attention.Scope) ([]notifications.OperatorNotice, error)
}

func (h *BaseHandlers) queuedOperatorNotificationItems(
	ctx context.Context, scope attention.Scope, labels map[string]string,
) ([]contract.AttentionNotificationPayload, error) {
	reader, ok := h.Sessions.(operatorNoticeReader)
	if !ok {
		return nil, nil
	}
	notices, err := reader.ListUnreadOperatorNotices(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("api: read queued operator notices: %w", err)
	}
	items := make([]contract.AttentionNotificationPayload, 0, len(notices))
	for _, notice := range notices {
		if _, registered := labels[notice.WorkspaceID]; notice.WorkspaceID != "" && !registered {
			continue
		}
		items = append(items, contract.AttentionNotificationPayload{
			ID: notice.NotificationID, Kind: "operator-notice", SourceID: notice.SessionID,
			WorkspaceID: notice.WorkspaceID, Title: notice.Title, Detail: notice.Body, OccurredAt: notice.At,
		})
	}
	return items, nil
}
