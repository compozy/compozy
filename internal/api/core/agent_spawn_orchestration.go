package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/worktree"
)

const (
	spawnWorktreeRollbackTimeout = 30 * time.Second
	spawnPromptErrorMaxBytes     = 2048
)

// ErrSpawnWorktreeConflict reports a spawn request naming both worktree selectors.
var ErrSpawnWorktreeConflict = fmt.Errorf(
	"%w: worktree and new_worktree are mutually exclusive",
	session.ErrSpawnValidation,
)

// SpawnWorktreeTarget is the worktree resolved or created for one spawned child.
type SpawnWorktreeTarget struct {
	Item        *worktree.Worktree
	WorkspaceID string
	Created     bool
}

// ID returns the bound worktree ID, or empty when the child is not bound to one.
func (t SpawnWorktreeTarget) ID() string {
	if t.Item == nil {
		return ""
	}
	return t.Item.ID
}

// ResolveSpawnWorktree resolves an existing worktree ref or creates the requested
// managed worktree in the child workspace. Callers roll a created worktree back with
// RollbackSpawnWorktree when the spawn fails.
func ResolveSpawnWorktree(
	ctx context.Context,
	svc WorktreeService,
	workspaceID string,
	profileID string,
	ref string,
	spec *contract.SpawnNewWorktreeRequest,
) (SpawnWorktreeTarget, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" && spec == nil {
		return SpawnWorktreeTarget{}, nil
	}
	if ref != "" && spec != nil {
		return SpawnWorktreeTarget{}, ErrSpawnWorktreeConflict
	}
	if svc == nil {
		return SpawnWorktreeTarget{}, errors.New("api: worktree service is unavailable")
	}
	if ref != "" {
		item, err := svc.Resolve(ctx, workspaceID, ref)
		if err != nil {
			return SpawnWorktreeTarget{}, err
		}
		return SpawnWorktreeTarget{Item: item, WorkspaceID: workspaceID}, nil
	}
	item, err := svc.CreateReady(ctx, workspaceID, worktree.CreateOptions{
		ProfileID: strings.TrimSpace(profileID),
		Name:      strings.TrimSpace(spec.Name),
		BaseRef:   strings.TrimSpace(spec.BaseRef),
		Branch:    strings.TrimSpace(spec.Branch),
		Origin:    worktree.OriginManual,
	})
	if err != nil {
		return SpawnWorktreeTarget{}, err
	}
	return SpawnWorktreeTarget{Item: item, WorkspaceID: workspaceID, Created: true}, nil
}

// RollbackSpawnWorktree removes a worktree created for a spawn that did not complete.
func RollbackSpawnWorktree(ctx context.Context, svc WorktreeService, target SpawnWorktreeTarget) error {
	if !target.Created || target.Item == nil || svc == nil {
		return nil
	}
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), spawnWorktreeRollbackTimeout)
	defer cancel()
	if _, err := svc.Remove(rollbackCtx, target.WorkspaceID, target.Item.ID, true); err != nil {
		return fmt.Errorf("api: roll back spawn worktree: %w", err)
	}
	return nil
}

// SpawnWorktreePayloadFromTarget projects the bound worktree of a spawned child.
func SpawnWorktreePayloadFromTarget(target SpawnWorktreeTarget) *contract.SpawnWorktreePayload {
	if target.Item == nil {
		return nil
	}
	return &contract.SpawnWorktreePayload{
		ID:      target.Item.ID,
		Name:    target.Item.Name,
		Branch:  target.Item.Branch,
		Path:    target.Item.Path,
		BaseSHA: target.Item.CreatedHead,
		Created: target.Created,
	}
}

// SpawnPromptPayloadFromResult projects the admitted first prompt of a spawned child.
func SpawnPromptPayloadFromResult(result session.SendPromptResult) *contract.SpawnPromptPayload {
	payload := &contract.SpawnPromptPayload{MessageID: result.MessageID, Status: result.Status}
	if result.ReplyWatch != nil {
		payload.ReplyWatchID = result.ReplyWatch.ID
		payload.ReplyWatchState = result.ReplyWatch.State
	}
	return payload
}

// SpawnPromptStatusFailed marks a first prompt that failed after its child was created.
const SpawnPromptStatusFailed = "failed"

// SpawnPromptFailurePayload reports a first prompt that failed after the child was
// created. The child keeps running so the caller can retry the prompt instead of
// spawning a duplicate child (and worktree).
func SpawnPromptFailurePayload(childSessionID string, err error) *contract.SpawnPromptPayload {
	return &contract.SpawnPromptPayload{
		MessageID: session.SpawnPromptMessageID(childSessionID),
		Status:    SpawnPromptStatusFailed,
		Error:     diagnostics.RedactAndBound(err.Error(), spawnPromptErrorMaxBytes),
	}
}

// SpawnPermissionNarrowing keeps absent (nil) categories distinct from empty ones so
// inherit_permissions can inherit only the categories the request omits.
func SpawnPermissionNarrowing(
	tools, skills, mcpServers, workspacePaths []string,
) *session.SubagentPermissionNarrowing {
	if tools == nil && skills == nil && mcpServers == nil && workspacePaths == nil {
		return nil
	}
	return &session.SubagentPermissionNarrowing{
		Tools:          trimSpawnAtoms(tools),
		Skills:         trimSpawnAtoms(skills),
		MCPServers:     trimSpawnAtoms(mcpServers),
		WorkspacePaths: trimSpawnAtoms(workspacePaths),
	}
}

func trimSpawnAtoms(values []string) []string {
	if values == nil {
		return nil
	}
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		trimmed = append(trimmed, strings.TrimSpace(value))
	}
	return trimmed
}
