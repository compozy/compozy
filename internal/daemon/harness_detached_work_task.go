package daemon

import (
	"context"

	"encoding/json"
	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (b *harnessDetachedWorkBridge) lookupDetachedHarnessSession(
	ctx context.Context,
	sessionID string,
	field string,
) (*session.Info, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("%w: detached harness %s is required", taskpkg.ErrValidation, field)
	}

	info, err := b.sessions.Status(ctx, sessionID)
	if err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			return nil, fmt.Errorf("%w: detached harness %s %q was not found", taskpkg.ErrValidation, field, sessionID)
		}
		return nil, err
	}
	if info == nil {
		return nil, fmt.Errorf("%w: detached harness %s %q was not found", taskpkg.ErrValidation, field, sessionID)
	}
	return info, nil
}

func normalizeDetachedHarnessWorkspace(
	scope taskpkg.Scope,
	workspaceID string,
	ownerInfo *session.Info,
	wakeInfo *session.Info,
) (string, error) {
	switch scope {
	case taskpkg.ScopeWorkspace:
		wakeWorkspaceID := strings.TrimSpace(wakeInfo.WorkspaceID)
		ownerWorkspaceID := strings.TrimSpace(ownerInfo.WorkspaceID)
		if wakeWorkspaceID == "" {
			wakeWorkspaceID = ownerWorkspaceID
		}
		if wakeWorkspaceID == "" {
			return "", fmt.Errorf(
				"%w: detached harness workspace scope requires a workspace-bound wake target",
				taskpkg.ErrValidation,
			)
		}
		if ownerWorkspaceID != "" && ownerWorkspaceID != wakeWorkspaceID {
			return "", fmt.Errorf(
				"%w: detached harness owner session %q and wake target %q must share one workspace",
				taskpkg.ErrValidation,
				strings.TrimSpace(ownerInfo.ID),
				strings.TrimSpace(wakeInfo.ID),
			)
		}
		if workspaceID != "" && workspaceID != wakeWorkspaceID {
			return "", fmt.Errorf(
				"%w: detached harness workspace %q does not match wake target workspace %q",
				taskpkg.ErrValidation,
				workspaceID,
				wakeWorkspaceID,
			)
		}
		return wakeWorkspaceID, nil
	case taskpkg.ScopeGlobal:
		if workspaceID != "" {
			return "", fmt.Errorf(
				"%w: detached harness global scope cannot include workspace_id",
				taskpkg.ErrValidation,
			)
		}
		return "", nil
	default:
		return "", fmt.Errorf("%w: unsupported detached harness scope %q", taskpkg.ErrValidation, scope)
	}
}

func (b *harnessDetachedWorkBridge) lookupExistingRun(
	ctx context.Context,
	req normalizedDetachedHarnessSubmitRequest,
	origin taskpkg.Origin,
	expected detachedHarnessRunMetadata,
) (taskpkg.Run, bool, error) {
	run, err := b.store.GetTaskRunByIdempotencyKey(ctx, req.SubmissionKey, origin)
	switch {
	case errors.Is(err, taskpkg.ErrTaskRunIdempotencyNotFound):
		return taskpkg.Run{}, false, nil
	case err != nil:
		return taskpkg.Run{}, false, err
	}
	if err := validateDetachedHarnessRunMatch(run, req, origin, expected); err != nil {
		return taskpkg.Run{}, false, err
	}
	return run, true, nil
}

func (b *harnessDetachedWorkBridge) ensureTask(
	ctx context.Context,
	req normalizedDetachedHarnessSubmitRequest,
	actor taskpkg.ActorContext,
	expected detachedHarnessTaskMetadata,
	expectedJSON json.RawMessage,
) (taskpkg.Task, bool, error) {
	if current, err := b.store.GetTask(ctx, req.TaskID); err == nil {
		if err := validateDetachedHarnessTaskMatch(current, req, actor, expected); err != nil {
			return taskpkg.Task{}, false, err
		}
		return current, true, nil
	} else if !errors.Is(err, taskpkg.ErrTaskNotFound) {
		return taskpkg.Task{}, false, err
	}

	created, err := b.tasks.CreateTask(ctx, taskpkg.CreateTask{
		ID:          req.TaskID,
		Scope:       req.Scope,
		WorkspaceID: req.WorkspaceID,
		Title:       req.Summary,
		Description: req.Description,
		Owner: &taskpkg.Ownership{
			Kind: taskpkg.OwnerKindAgentSession,
			Ref:  req.OwnerSessionID,
		},
		Metadata: expectedJSON,
	}, actor)
	if err == nil {
		if created == nil {
			return taskpkg.Task{}, false, errors.New("daemon: detached harness submit returned empty task")
		}
		return *created, false, nil
	}

	current, getErr := b.store.GetTask(ctx, req.TaskID)
	if getErr != nil {
		return taskpkg.Task{}, false, err
	}
	if err := validateDetachedHarnessTaskMatch(current, req, actor, expected); err != nil {
		return taskpkg.Task{}, false, err
	}
	return current, true, nil
}

func buildDetachedHarnessTaskMetadata(req normalizedDetachedHarnessSubmitRequest) detachedHarnessTaskMetadata {
	return detachedHarnessTaskMetadata{
		Schema:               harnessDetachedMetadataSchema,
		Kind:                 harnessDetachedTaskMetadataKey,
		SubmissionKey:        req.SubmissionKey,
		Summary:              req.Summary,
		SubmissionTurnSource: string(req.TurnSource),
		OwnerSessionID:       req.OwnerSessionID,
		OwnerSessionType:     req.OwnerSessionType,
		OwnerWorkspaceID:     req.OwnerWorkspaceID,
		WakeTarget:           req.WakeTarget,
	}
}

func buildDetachedHarnessRunMetadata(req normalizedDetachedHarnessSubmitRequest) detachedHarnessRunMetadata {
	return detachedHarnessRunMetadata{
		Schema:               harnessDetachedMetadataSchema,
		Kind:                 harnessDetachedRunMetadataKey,
		SubmissionKey:        req.SubmissionKey,
		Summary:              req.Summary,
		SubmissionTurnSource: string(req.TurnSource),
		OwnerSessionID:       req.OwnerSessionID,
		OwnerSessionType:     req.OwnerSessionType,
		OwnerWorkspaceID:     req.OwnerWorkspaceID,
		WakeTarget:           req.WakeTarget,
		Reentry:              nil,
	}
}

func marshalDetachedHarnessMetadata(value any) (json.RawMessage, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("daemon: marshal detached harness metadata: %w", err)
	}
	return payload, nil
}
