package daemon

import (
	"context"
	"errors"

	core "github.com/compozy/agh/internal/api/core"

	"github.com/compozy/agh/internal/heartbeat"
)

func hookHeartbeatAuthoringService(
	next core.HeartbeatAuthoringService,
	hooks *hooksNotifier,
) core.HeartbeatAuthoringService {
	if next == nil {
		return nil
	}
	return hookedHeartbeatAuthoringService{next: next, hooks: hooks}
}

type hookedHeartbeatAuthoringService struct {
	next  core.HeartbeatAuthoringService
	hooks *hooksNotifier
}

func (s hookedHeartbeatAuthoringService) Validate(
	ctx context.Context,
	req heartbeat.ValidateRequest,
) (heartbeat.ValidateResult, error) {
	if s.next == nil {
		return heartbeat.ValidateResult{}, errors.New("daemon: heartbeat authoring service is not configured")
	}
	result, err := s.next.Validate(ctx, req)
	if err == nil {
		s.dispatchHeartbeatPolicyResolved(ctx, req.Target, &result.Policy, "")
	}
	return result, err
}

func (s hookedHeartbeatAuthoringService) Put(
	ctx context.Context,
	req heartbeat.PutRequest,
) (heartbeat.MutationResult, error) {
	if s.next == nil {
		return heartbeat.MutationResult{}, errors.New("daemon: heartbeat authoring service is not configured")
	}
	return s.next.Put(ctx, req)
}

func (s hookedHeartbeatAuthoringService) Delete(
	ctx context.Context,
	req heartbeat.DeleteRequest,
) (heartbeat.MutationResult, error) {
	if s.next == nil {
		return heartbeat.MutationResult{}, errors.New("daemon: heartbeat authoring service is not configured")
	}
	return s.next.Delete(ctx, req)
}

func (s hookedHeartbeatAuthoringService) History(
	ctx context.Context,
	req heartbeat.HistoryRequest,
) (heartbeat.HistoryResult, error) {
	if s.next == nil {
		return heartbeat.HistoryResult{}, errors.New("daemon: heartbeat authoring service is not configured")
	}
	return s.next.History(ctx, req)
}

func (s hookedHeartbeatAuthoringService) Rollback(
	ctx context.Context,
	req heartbeat.RollbackRequest,
) (heartbeat.MutationResult, error) {
	if s.next == nil {
		return heartbeat.MutationResult{}, errors.New("daemon: heartbeat authoring service is not configured")
	}
	return s.next.Rollback(ctx, req)
}

func (s hookedHeartbeatAuthoringService) dispatchHeartbeatPolicyResolved(
	ctx context.Context,
	target heartbeat.AuthoringTarget,
	policy *heartbeat.ResolvedPolicy,
	snapshotID string,
) {
	dispatchHeartbeatPolicyResolved(ctx, s.hooks, target.WorkspaceID, target.AgentName, policy, snapshotID)
}

func hookHeartbeatStatusService(
	next core.HeartbeatStatusService,
	hooks *hooksNotifier,
) core.HeartbeatStatusService {
	if next == nil {
		return nil
	}
	return hookedHeartbeatStatusService{next: next, hooks: hooks}
}

type hookedHeartbeatStatusService struct {
	next  core.HeartbeatStatusService
	hooks *hooksNotifier
}

func (s hookedHeartbeatStatusService) Inspect(
	ctx context.Context,
	req heartbeat.InspectRequest,
) (heartbeat.InspectResult, error) {
	if s.next == nil {
		return heartbeat.InspectResult{}, errors.New("daemon: heartbeat status service is not configured")
	}
	result, err := s.next.Inspect(ctx, req)
	if err == nil {
		snapshotID := ""
		if result.Snapshot != nil {
			snapshotID = result.Snapshot.ID
		}
		dispatchHeartbeatPolicyResolved(
			ctx,
			s.hooks,
			req.Target.WorkspaceID,
			result.AgentName,
			&result.Policy,
			snapshotID,
		)
	}
	return result, err
}

func (s hookedHeartbeatStatusService) Status(
	ctx context.Context,
	req heartbeat.StatusRequest,
) (heartbeat.StatusResult, error) {
	if s.next == nil {
		return heartbeat.StatusResult{}, errors.New("daemon: heartbeat status service is not configured")
	}
	result, err := s.next.Status(ctx, req)
	if err == nil {
		policy := heartbeat.ResolvedPolicy{
			Enabled:          result.Enabled,
			Present:          result.Present,
			Active:           result.Active,
			Valid:            result.Valid,
			SourcePath:       result.SourcePath,
			Digest:           result.Digest,
			ConfigDigest:     result.ConfigDigest,
			Summary:          result.Summary,
			ConfigProvenance: result.ConfigProvenance,
			Preferences:      result.Preferences,
			Diagnostics:      result.Diagnostics,
		}
		dispatchHeartbeatPolicyResolved(
			ctx,
			s.hooks,
			req.Target.WorkspaceID,
			result.AgentName,
			&policy,
			result.SnapshotID,
		)
	}
	return result, err
}
