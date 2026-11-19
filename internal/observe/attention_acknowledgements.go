package observe

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/observe/attention"
	"github.com/compozy/compozy/internal/store"
)

var _ attention.Store = (*Observer)(nil)

func (o *Observer) ListUnreadAutomationFailures(
	ctx context.Context, scope attention.Scope, readScope store.ReadScope, workspaceIDs []string,
) ([]attention.AutomationFailure, error) {
	reader, ok := o.registry.(attention.AutomationFailureReader)
	if !ok {
		return nil, errors.New("observe: automation failure source is unavailable")
	}
	return reader.ListUnreadAutomationFailures(ctx, scope, readScope, workspaceIDs)
}

func (o *Observer) AttentionSnapshotOccurrences(
	ctx context.Context, scope attention.Scope, snapshot, occurrence string,
) ([]string, error) {
	store, ok := o.registry.(attention.Store)
	if !ok {
		return nil, errors.New("observe: attention receipt store is unavailable")
	}
	return store.AttentionSnapshotOccurrences(ctx, scope, snapshot, occurrence)
}

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
