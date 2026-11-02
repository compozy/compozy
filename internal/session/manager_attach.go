package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
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
	meta, err := m.readMetaWithContext(ctx, normalized.SessionID)
	if err != nil {
		return store.SessionAttach{}, err
	}
	if err := m.rejectDeadSessionAttachment(ctx, normalized.SessionID, &meta); err != nil {
		return store.SessionAttach{}, err
	}
	attach, err := m.sessionCatalog.AttachSession(ctx, normalized)
	if err != nil {
		if errors.Is(err, store.ErrSessionNotAttachable) {
			resolved, resolveErr := m.resolveResumeWorkspace(ctx, &meta)
			if resolveErr == nil {
				_, agentErr := m.resolveWorkspaceAgentArtifactsForSession(
					meta.AgentName,
					Type(meta.SessionType),
					&resolved,
				)
				if errors.Is(agentErr, skills.ErrAgentNotFound) ||
					errors.Is(agentErr, workspacepkg.ErrAgentNotAvailable) {
					return store.SessionAttach{}, fmt.Errorf("%w: agent %q is no longer available", err, meta.AgentName)
				}
			}
			return store.SessionAttach{}, fmt.Errorf("%w: session %q is %s", err, normalized.SessionID, meta.State)
		}
		return store.SessionAttach{}, fmt.Errorf("session: attach %q: %w", normalized.SessionID, err)
	}
	if active, ok := m.Get(attach.SessionID); ok {
		active.applyAttach(attach)
	}
	return attach, nil
}
