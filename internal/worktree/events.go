package worktree

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/compozy/compozy/internal/redact"
)

const (
	EventPreCreate           = "worktree.pre_create"
	EventPreRemove           = "worktree.pre_remove"
	EventCreated             = "worktree.created"
	EventAdopted             = "worktree.adopted"
	EventRemoved             = "worktree.removed"
	EventMissing             = "worktree.missing"
	EventDismissed           = "worktree.dismissed"
	EventCreationCanceled    = "worktree.creation_canceled"
	EventSetupFailed         = "worktree.setup_failed"
	EventStatusRefreshed     = "worktree.status_refreshed"
	EventBranchReclaimed     = "worktree.branch_reclaimed"
	EventExitActionStarted   = "worktree.exit_action_started"
	EventExitActionStep      = "worktree.exit_action_step"
	EventExitActionCompleted = "worktree.exit_action_completed"
	EventExitActionFailed    = "worktree.exit_action_failed"
	EventExitActionCanceled  = "worktree.exit_action_canceled"
)

type HookRequest struct {
	Event   string
	Payload any
}

type HookVerdict struct {
	Denied   bool
	HookName string
	Reason   string
}

type HookDispatcher interface {
	DispatchWorktreeHook(context.Context, HookRequest) (HookVerdict, error)
}

type LifecycleEvent struct {
	Name        string
	WorkspaceID string
	WorktreeID  string
	Payload     json.RawMessage
}

type EventSink interface {
	PublishWorktreeEvent(context.Context, LifecycleEvent) error
}

type HookWorktree struct {
	WorktreeID    string `json:"worktree_id"`
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceRoot string `json:"workspace_root,omitempty"`
	Name          string `json:"name"`
	Branch        string `json:"branch"`
	Path          string `json:"path"`
	Origin        Origin `json:"origin"`
}

func (s *Service) dispatchGate(ctx context.Context, event string, payload any) error {
	if s.hooks == nil {
		return nil
	}
	verdict, err := s.hooks.DispatchWorktreeHook(ctx, HookRequest{Event: event, Payload: payload})
	if err != nil {
		s.logger.WarnContext(
			ctx, "worktree hook execution failed open", "hook_event", event,
			"error", redact.String(err.Error()),
		)
		return nil
	}
	if verdict.Denied {
		detail := strings.TrimSpace(verdict.HookName + ": " + verdict.Reason)
		return refusal(ErrDeniedByHook, redact.String(detail))
	}
	return nil
}

func (s *Service) emit(ctx context.Context, name string, item Worktree) {
	if s.events != nil {
		payload, err := json.Marshal(eventPayloadFromWorktree(item, ""))
		if err != nil {
			s.logger.ErrorContext(ctx, "worktree event payload failed", "event", name, "error", err)
		} else if err := s.events.PublishWorktreeEvent(ctx, LifecycleEvent{
			Name: name, WorkspaceID: item.WorkspaceID, WorktreeID: item.ID, Payload: payload,
		}); err != nil {
			s.logger.WarnContext(
				ctx, "worktree event dispatch failed open", "event", name,
				"error", redact.String(err.Error()),
			)
		}
	}
	if s.hooks != nil && (name == EventCreated || name == EventAdopted || name == EventRemoved) {
		workspaceRoot := ""
		if workspace, err := s.resolveWorkspace(ctx, item.WorkspaceID); err == nil {
			workspaceRoot = workspace.Root
		}
		if _, err := s.hooks.DispatchWorktreeHook(ctx, HookRequest{
			Event: name, Payload: eventPayloadFromWorktree(item, workspaceRoot),
		}); err != nil {
			s.logger.WarnContext(
				ctx, "worktree observation hook failed open", "hook_event", name,
				"error", redact.String(err.Error()),
			)
		}
	}
}
