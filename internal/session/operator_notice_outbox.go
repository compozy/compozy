package session

import (
	"context"

	"github.com/compozy/compozy/internal/notifications"
	"github.com/compozy/compozy/internal/observe/attention"
)

// OperatorNoticeOutbox retains notices that could not reach a live subscriber.
type OperatorNoticeOutbox interface {
	EnqueueOperatorNotice(context.Context, notifications.OperatorNotice) error
	ListUnreadOperatorNotices(context.Context, attention.Scope) ([]notifications.OperatorNotice, error)
}

func WithOperatorNoticeOutbox(outbox OperatorNoticeOutbox) Option {
	return func(m *Manager) {
		m.operatorNoticeOutbox = outbox
	}
}

func (m *Manager) ListUnreadOperatorNotices(
	ctx context.Context, scope attention.Scope,
) ([]notifications.OperatorNotice, error) {
	if m.operatorNoticeOutbox == nil {
		return nil, nil
	}
	return m.operatorNoticeOutbox.ListUnreadOperatorNotices(ctx, scope)
}
