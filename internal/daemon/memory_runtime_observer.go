package daemon

import (
	"context"
	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func (e *daemonMemoryExtractor) HandleSessionMessagePersisted(
	ctx context.Context,
	payload hookspkg.SessionMessagePersistedPayload,
) error {
	if e == nil || e.runtime == nil {
		return nil
	}
	if strings.TrimSpace(payload.ParentSessionID) != "" {
		return nil
	}
	if workspaceRoot := strings.TrimSpace(payload.Workspace); workspaceRoot != "" {
		sessionID := firstNonEmptyString(payload.SessionID, payload.RootSessionID)
		if sessionID != "" {
			e.workspaceRoots.Store(sessionID, workspaceRoot)
		}
	}
	return e.runtime.HandleSessionMessagePersisted(ctx, payload)
}

func (e *daemonMemoryExtractor) RecordToolWrite(sessionID string, turnSeq int64) {
	if e == nil || e.runtime == nil {
		return
	}
	e.runtime.RecordToolWrite(sessionID, turnSeq)
}
