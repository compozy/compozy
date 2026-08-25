package daemon

import (
	"context"
	"strings"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

func (r *spawnReaper) dispatchReasonHook(ctx context.Context, candidate spawnReapCandidate) {
	payload := r.spawnLifecyclePayload(candidate, nil)
	var err error
	switch candidate.reason {
	case spawnReapReasonTTLExpired:
		payload.Event = hookspkg.HookSpawnTTLExpired
		_, err = r.hooksOrNoop().DispatchSpawnTTLExpired(ctx, payload)
	case spawnReapReasonParentStopped:
		payload.Event = hookspkg.HookSpawnParentStopped
		_, err = r.hooksOrNoop().DispatchSpawnParentStopped(ctx, payload)
	}
	if err != nil && r.logger != nil {
		r.logger.Warn("daemon: spawn lifecycle hook failed", "event", payload.Event, "error", err)
	}
}

func (r *spawnReaper) dispatchReapedHook(ctx context.Context, candidate spawnReapCandidate, reapErr error) {
	payload := r.spawnLifecyclePayload(candidate, reapErr)
	payload.Event = hookspkg.HookSpawnReaped
	if _, err := r.hooksOrNoop().DispatchSpawnReaped(ctx, payload); err != nil &&
		r.logger != nil {
		r.logger.Warn("daemon: spawn reaped hook failed", "error", err)
	}
}

func (r *spawnReaper) spawnLifecyclePayload(
	candidate spawnReapCandidate,
	reapErr error,
) hookspkg.SpawnLifecyclePayload {
	child := candidate.child
	lineage := store.NormalizeSessionLineage("", nil)
	if child != nil {
		lineage = store.NormalizeSessionLineage(child.ID, child.Lineage)
	}
	payload := hookspkg.SpawnLifecyclePayload{
		PayloadBase: hookspkg.PayloadBase{Timestamp: r.now().UTC()},
		SpawnContext: hookspkg.SpawnContext{
			ParentSessionID:  lineage.ParentSessionID,
			RootSessionID:    lineage.RootSessionID,
			SpawnDepth:       lineage.SpawnDepth,
			SpawnRole:        lineage.SpawnRole,
			TTLSeconds:       lineage.SpawnBudget.TTLSeconds,
			AutoStopOnParent: lineage.AutoStopOnParent,
		},
		ChildPermissions: spawnReaperPermissionSet(lineage.PermissionPolicy),
		StopReason:       candidate.reason,
		ReapReason:       candidate.reason,
	}
	if candidate.parent != nil {
		payload.ProfileID = strings.TrimSpace(candidate.parent.ProfileID)
	}
	if child != nil {
		payload.ProfileID = strings.TrimSpace(child.ProfileID)
		payload.ChildSessionID = child.ID
		payload.AgentName = child.AgentName
		payload.WorkspaceID = child.WorkspaceID
		payload.Workspace = child.Workspace
		payload.ResolvedNetworkParticipation = participation.CloneSpec(child.NetworkParticipation)
		payload.SoulSnapshotID = child.SoulSnapshotID
		payload.SoulDigest = child.SoulDigest
		payload.ParentSoulDigest = child.ParentSoulDigest
	}
	if candidate.parent != nil {
		if payload.ResolvedNetworkParticipation == nil {
			payload.ResolvedNetworkParticipation = participation.CloneSpec(
				candidate.parent.NetworkParticipation,
			)
		}
		if strings.TrimSpace(payload.ParentSoulDigest) == "" {
			payload.ParentSoulDigest = candidate.parent.SoulDigest
		}
		if candidate.parent.Lineage != nil {
			payload.ParentPermissions = spawnReaperPermissionSet(candidate.parent.Lineage.PermissionPolicy)
		}
	}
	if reapErr != nil {
		payload.Error = reapErr.Error()
	}
	return payload
}

func (r *spawnReaper) hooksOrNoop() session.SpawnHooks {
	if r == nil || r.hooks == nil {
		return spawnReaperNoopHooks{}
	}
	return r.hooks
}

func spawnReaperPermissionSet(policy store.SessionPermissionPolicy) *hookspkg.PermissionSet {
	normalized := store.NormalizeSessionPermissionPolicy(policy)
	return &hookspkg.PermissionSet{
		Tools:           append([]string(nil), normalized.Tools...),
		Skills:          append([]string(nil), normalized.Skills...),
		MCPServers:      append([]string(nil), normalized.MCPServers...),
		WorkspacePaths:  append([]string(nil), normalized.WorkspacePaths...),
		NetworkChannels: append([]string(nil), normalized.NetworkChannels...),
		SandboxProfiles: append([]string(nil), normalized.SandboxProfiles...),
	}
}

func spawnReaperLiveState(state session.State) bool {
	switch state {
	case session.StateStarting, session.StateActive, session.StateStopping:
		return true
	default:
		return false
	}
}

type spawnReaperNoopHooks struct{}

func (spawnReaperNoopHooks) DispatchSpawnPreCreate(
	_ context.Context,
	payload hookspkg.SpawnPreCreatePayload,
) (hookspkg.SpawnPreCreatePayload, error) {
	return payload, nil
}

func (spawnReaperNoopHooks) DispatchSpawnCreated(
	_ context.Context,
	payload hookspkg.SpawnCreatedPayload,
) (hookspkg.SpawnCreatedPayload, error) {
	return payload, nil
}

func (spawnReaperNoopHooks) DispatchSpawnParentStopped(
	_ context.Context,
	payload hookspkg.SpawnParentStoppedPayload,
) (hookspkg.SpawnParentStoppedPayload, error) {
	return payload, nil
}

func (spawnReaperNoopHooks) DispatchSpawnTTLExpired(
	_ context.Context,
	payload hookspkg.SpawnTTLExpiredPayload,
) (hookspkg.SpawnTTLExpiredPayload, error) {
	return payload, nil
}

func (spawnReaperNoopHooks) DispatchSpawnReaped(
	_ context.Context,
	payload hookspkg.SpawnReapedPayload,
) (hookspkg.SpawnReapedPayload, error) {
	return payload, nil
}
