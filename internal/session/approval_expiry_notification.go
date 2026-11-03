package session

import (
	"context"

	"github.com/compozy/compozy/internal/acp"
)

func (m *Manager) notifyApprovalExpiry(ctx context.Context, current *Session, event acp.AgentEvent) {
	expired := event.ResolvedByValue() == "timeout" ||
		(event.ResolvedByValue() == "provider" && event.Decision == string(acp.PermissionDecisionCanceled))
	if event.Type != acp.EventTypePermission || !expired || current == nil {
		return
	}
	info := current.Info()
	_, err := m.publishOperatorNotification(ctx, NotifyRequest{
		Kind: "approval_expired", Action: event.Action, Resource: event.Resource,
		SessionID: info.ID, ProfileID: info.ProfileID, WorkspaceID: info.WorkspaceID,
		Title: "Approval expired", Body: "Nobody answered before the deadline. The action was not approved.",
	}, false)
	if err != nil {
		m.sessionLogger(current).WarnContext(ctx, "session: approval expiry notification failed",
			"request_id", event.RequestIDValue(), "error", err)
	}
}
