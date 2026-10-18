package session

import (
	"context"
	"errors"

	"fmt"
	"strings"
	"time"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) dispatchSpawnPreCreate(
	ctx context.Context,
	parent *Info,
	opts SpawnOpts,
	lineage *store.SessionLineage,
) (SpawnOpts, *store.SessionLineage, error) {
	expectedSubagent := opts.Subagent
	if expectedSubagent != nil {
		expected := *expectedSubagent
		expectedSubagent = &expected
	}
	payload := hookspkg.SpawnPreCreatePayload{
		Event:             hookspkg.HookSpawnPreCreate,
		Subagent:          opts.Subagent,
		Timestamp:         m.now().UTC(),
		SpawnContext:      spawnHookContext(parent, nil, lineage, opts.AgentName, opts.SpawnRole),
		ParentPermissions: hookPermissionSetFromPolicy(parent.Lineage.PermissionPolicy),
		ChildPermissions:  hookPermissionSetFromPolicy(opts.PermissionPolicy),
	}
	result, err := m.hooks.spawn().DispatchSpawnPreCreate(ctx, payload)
	if err != nil {
		if denial, denied := errors.AsType[*hookspkg.DeniedError](
			err,
		); denied &&
			opts.SpawnRole == store.SubagentSpawnRole {
			return SpawnOpts{}, nil, &SubagentError{
				Code:    subagentCapabilityDeniedCode,
				Message: denial.Reason,
				Err:     errors.Join(ErrSubagentCapabilityDenied, err),
			}
		}
		return SpawnOpts{}, nil, fmt.Errorf("%w: %w", ErrSpawnPermissionDenied, err)
	}
	if result.Denied {
		reason := strings.TrimSpace(result.DenyReason)
		if reason == "" {
			reason = "spawn denied by hook"
		}
		if opts.SpawnRole == store.SubagentSpawnRole {
			return SpawnOpts{}, nil, &SubagentError{
				Code:    subagentCapabilityDeniedCode,
				Message: reason,
				Err:     ErrSubagentCapabilityDenied,
			}
		}
		return SpawnOpts{}, nil, fmt.Errorf("%w: %s", ErrSpawnPermissionDenied, reason)
	}

	if expectedSubagent != nil &&
		(result.Subagent == nil || result.Subagent.Isolation != expectedSubagent.Isolation ||
			result.Subagent.WorktreeID != expectedSubagent.WorktreeID) {
		return SpawnOpts{}, nil, &SubagentError{
			Code:    subagentCapabilityDeniedCode,
			Message: "inherited worktree binding is immutable",
			Err:     ErrSubagentCapabilityDenied,
		}
	}
	if opts.SpawnRole == store.SubagentSpawnRole &&
		(result.TTLSeconds != 0 || result.SpawnRole != store.SubagentSpawnRole) {
		return SpawnOpts{}, nil, ErrSubagentCapabilityDenied
	}
	opts.AgentName = strings.TrimSpace(result.AgentName)
	opts.SpawnRole = normalizeSpawnRole(result.SpawnRole)
	opts.TTL = time.Duration(result.TTLSeconds) * time.Second
	opts.PermissionPolicy = policyFromHookPermissionSet(result.ChildPermissions)
	normalized, err := normalizeSpawnOpts(opts)
	if err != nil {
		return SpawnOpts{}, nil, err
	}
	lineage, err = m.spawnLineage(ctx, parent, normalized)
	if err != nil {
		return SpawnOpts{}, nil, err
	}
	return normalized, lineage, nil
}

func (m *Manager) dispatchSpawnCreated(ctx context.Context, parent *Info, child *Info) error {
	if parent == nil || child == nil || child.Lineage == nil {
		return nil
	}
	lineage := store.NormalizeSessionLineage(child.ID, child.Lineage)
	payload := hookspkg.SpawnCreatedPayload{
		Event:             hookspkg.HookSpawnCreated,
		Timestamp:         m.now().UTC(),
		SpawnContext:      spawnHookContext(parent, child, lineage, child.AgentName, lineage.SpawnRole),
		ParentPermissions: hookPermissionSetFromPolicy(parent.Lineage.PermissionPolicy),
		ChildPermissions:  hookPermissionSetFromPolicy(lineage.PermissionPolicy),
	}
	_, err := m.hooks.spawn().DispatchSpawnCreated(ctx, payload)
	return err
}

func spawnHookContext(
	parent *Info,
	child *Info,
	lineage *store.SessionLineage,
	agentName string,
	spawnRole string,
) hookspkg.SpawnContext {
	ctx := hookspkg.SpawnContext{
		AgentName:        strings.TrimSpace(agentName),
		SpawnRole:        strings.TrimSpace(spawnRole),
		ParentSessionID:  strings.TrimSpace(lineage.ParentSessionID),
		RootSessionID:    strings.TrimSpace(lineage.RootSessionID),
		SpawnDepth:       lineage.SpawnDepth,
		AutoStopOnParent: lineage.AutoStopOnParent,
		TTLSeconds:       lineage.SpawnBudget.TTLSeconds,
	}
	if parent != nil {
		ctx.ProfileID = strings.TrimSpace(parent.ProfileID)
		ctx.WorkspaceID = strings.TrimSpace(parent.WorkspaceID)
		ctx.Workspace = strings.TrimSpace(parent.Workspace)
		ctx.ParentSoulDigest = strings.TrimSpace(parent.SoulDigest)
	}
	if child != nil {
		ctx.ProfileID = strings.TrimSpace(child.ProfileID)
		ctx.ChildSessionID = strings.TrimSpace(child.ID)
		ctx.WorkspaceID = strings.TrimSpace(child.WorkspaceID)
		ctx.Workspace = strings.TrimSpace(child.Workspace)
		ctx.SoulSnapshotID = strings.TrimSpace(child.SoulSnapshotID)
		ctx.SoulDigest = strings.TrimSpace(child.SoulDigest)
		if value := strings.TrimSpace(child.ParentSoulDigest); value != "" {
			ctx.ParentSoulDigest = value
		}
	}
	return ctx
}
