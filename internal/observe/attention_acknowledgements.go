package observe

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/notifications"
)

var _ notifications.AttentionStore = (*Observer)(nil)

func (o *Observer) CaptureAttentionSnapshot(
	ctx context.Context,
	scope notifications.AttentionScope,
	ids []string,
) (string, []string, error) {
	store, ok := o.registry.(notifications.AttentionStore)
	if !ok {
		return "", nil, errors.New("observe: attention receipt store is unavailable")
	}
	return store.CaptureAttentionSnapshot(ctx, scope, ids)
}

func (o *Observer) AcknowledgeAttentionSnapshot(
	ctx context.Context,
	scope notifications.AttentionScope,
	snapshot, occurrence string,
) error {
	store, ok := o.registry.(notifications.AttentionStore)
	if !ok {
		return errors.New("observe: attention receipt store is unavailable")
	}
	return store.AcknowledgeAttentionSnapshot(ctx, scope, snapshot, occurrence)
}
