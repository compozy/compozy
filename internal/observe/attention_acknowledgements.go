package observe

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/observe/attention"
)

var _ attention.Store = (*Observer)(nil)

func (o *Observer) CaptureAttentionSnapshot(
	ctx context.Context,
	scope attention.Scope,
	ids []string,
) (string, []string, error) {
	store, ok := o.registry.(attention.Store)
	if !ok {
		return "", nil, errors.New("observe: attention receipt store is unavailable")
	}
	return store.CaptureAttentionSnapshot(ctx, scope, ids)
}

func (o *Observer) AcknowledgeAttentionSnapshot(
	ctx context.Context,
	scope attention.Scope,
	snapshot, occurrence string,
) error {
	store, ok := o.registry.(attention.Store)
	if !ok {
		return errors.New("observe: attention receipt store is unavailable")
	}
	return store.AcknowledgeAttentionSnapshot(ctx, scope, snapshot, occurrence)
}
