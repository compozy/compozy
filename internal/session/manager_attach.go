package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/store"
)

// AttachSession acquires the durable attach lock before updating the live session snapshot.
func (m *Manager) AttachSession(
	ctx context.Context,
	req store.SessionAttachRequest,
) (store.SessionAttach, error) {
	if m == nil {
		return store.SessionAttach{}, errors.New("session: manager is required")
	}
	if ctx == nil {
		return store.SessionAttach{}, errors.New("session: attach context is required")
	}
	normalized := req.Normalize()
	if err := normalized.Validate(); err != nil {
		return store.SessionAttach{}, err
	}
	if m.sessionCatalog == nil {
		return store.SessionAttach{}, errors.New("session: attach catalog is required")
	}
	attach, err := m.sessionCatalog.AttachSession(ctx, normalized)
	if err != nil {
		return store.SessionAttach{}, fmt.Errorf("session: attach %q: %w", normalized.SessionID, err)
	}
	if active, ok := m.Get(attach.SessionID); ok {
		active.applyAttach(attach)
	}
	return attach, nil
}
