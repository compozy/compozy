package workspace

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	InvitationScopeWorkspace = "workspace"
	InvitationScopeTask      = "task"
)

// CoordinationInvitations owns daemon-persisted invitation dismissal state.
type CoordinationInvitations interface {
	GetInvitation(
		ctx context.Context,
		workspaceID string,
		scopeKind string,
		scopeID string,
	) (CoordinationInvitation, error)
	DismissInvitation(
		ctx context.Context,
		workspaceID string,
		scopeKind string,
		scopeID string,
		actor string,
	) (CoordinationInvitation, error)
	ResetInvitation(ctx context.Context, workspaceID string, scopeKind string, scopeID string) error
}

// CoordinationInvitation is one persisted dismissal row. Absent rows mean not dismissed.
type CoordinationInvitation struct {
	WorkspaceID string
	ScopeKind   string
	ScopeID     string
	Dismissed   bool
	DismissedAt time.Time
	DismissedBy string
}

// NormalizeInvitationScope validates and returns scope_kind plus durable scope_id.
func NormalizeInvitationScope(
	scope string,
	workspaceID string,
	taskID string,
) (scopeKind string, scopeID string, err error) {
	normalized := strings.TrimSpace(strings.ToLower(scope))
	ws := strings.TrimSpace(workspaceID)
	task := strings.TrimSpace(taskID)
	switch normalized {
	case InvitationScopeWorkspace:
		if ws == "" {
			return "", "", fmt.Errorf("workspace: invitation workspace_id is required")
		}
		if task != "" {
			return "", "", fmt.Errorf("workspace: task_id must be empty for workspace invitation scope")
		}
		return InvitationScopeWorkspace, ws, nil
	case InvitationScopeTask:
		if task == "" {
			return "", "", fmt.Errorf("workspace: invitation task_id is required for task scope")
		}
		return InvitationScopeTask, task, nil
	default:
		return "", "", fmt.Errorf("workspace: invitation scope must be workspace or task")
	}
}
