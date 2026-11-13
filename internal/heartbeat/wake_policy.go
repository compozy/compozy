package heartbeat

import (
	"context"
	"fmt"
)

func (s *ManagedWakeService) currentWakeSnapshot(ctx context.Context, req WakeRequest) (Snapshot, error) {
	if req.PolicyTarget == nil {
		return s.store.GetLatestValidHeartbeatSnapshot(ctx, req.WorkspaceID, req.AgentName, req.ProfileID)
	}
	status := ManagedHeartbeatStatusService{policyResolver: s.policyResolver}
	target, policy, err := status.resolvePolicy(ctx, *req.PolicyTarget)
	if err != nil {
		return Snapshot{}, err
	}
	if !policy.Valid {
		return Snapshot{}, authoringInvalidError(policy.Diagnostics)
	}
	if !policy.Present {
		return Snapshot{}, ErrSnapshotNotFound
	}
	if req.DryRun {
		if finder, ok := s.store.(interface {
			FindHeartbeatSnapshotByDigest(context.Context, string, string, string, string) (Snapshot, bool, error)
		}); ok {
			stored, found, err := finder.FindHeartbeatSnapshotByDigest(
				ctx, target.workspaceID, target.agentName, policy.Digest, target.profileID,
			)
			if err != nil {
				return Snapshot{}, fmt.Errorf("heartbeat: find wake policy snapshot: %w", err)
			}
			if found {
				return stored, nil
			}
		}
	}
	id, err := s.newID("hb")
	if err != nil {
		return Snapshot{}, fmt.Errorf("heartbeat: generate policy snapshot id: %w", err)
	}
	snapshot, err := SnapshotFromResolved(
		id, target.workspaceID, target.agentName, &policy, s.currentTime(), target.profileID,
	)
	if err != nil {
		return Snapshot{}, err
	}
	if req.DryRun {
		snapshot.ID = ""
		return snapshot, nil
	}
	writer, ok := s.store.(interface {
		UpsertHeartbeatSnapshot(context.Context, Snapshot) (Snapshot, error)
	})
	if !ok {
		return Snapshot{}, fmt.Errorf("heartbeat: policy snapshot persistence is unavailable")
	}
	return writer.UpsertHeartbeatSnapshot(ctx, snapshot)
}
