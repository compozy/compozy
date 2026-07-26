package situation

import (
	"context"

	"strings"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/session"

	"github.com/compozy/agh/internal/soul"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (s *Service) soulPayload(ctx context.Context, info *session.Info) (contract.AgentSoulSectionPayload, error) {
	if info == nil || strings.TrimSpace(info.SoulSnapshotID) == "" {
		return contract.AgentSoulSectionPayload{}, nil
	}
	store := s.soulSnapshotsValue()
	if store == nil {
		return contract.AgentSoulSectionPayload{
			Present:    true,
			Active:     true,
			Valid:      true,
			SnapshotID: strings.TrimSpace(info.SoulSnapshotID),
			Digest:     strings.TrimSpace(info.SoulDigest),
		}, nil
	}
	snapshot, err := store.GetSoulSnapshot(ctx, info.SoulSnapshotID)
	if err != nil {
		if isContextError(err) {
			return contract.AgentSoulSectionPayload{}, err
		}
		return contract.AgentSoulSectionPayload{
			Present:    true,
			Active:     true,
			Valid:      false,
			SnapshotID: strings.TrimSpace(info.SoulSnapshotID),
			Digest:     strings.TrimSpace(info.SoulDigest),
		}, nil
	}
	return soulPayloadFromSnapshot(&snapshot), nil
}

func sessionPayload(info *session.Info) contract.AgentSessionPayload {
	if info == nil {
		return contract.AgentSessionPayload{}
	}
	return contract.AgentSessionPayload{
		ID:                           strings.TrimSpace(info.ID),
		Name:                         strings.TrimSpace(info.Name),
		Type:                         info.Type,
		State:                        info.State,
		ResolvedNetworkParticipation: normalizedSessionParticipation(info.NetworkParticipation),
		Lineage:                      contract.SessionLineagePayloadFromStore(info.Lineage),
		CreatedAt:                    info.CreatedAt.UTC(),
		UpdatedAt:                    info.UpdatedAt.UTC(),
	}
}

func soulPayloadFromSnapshot(snapshot *soul.Snapshot) contract.AgentSoulSectionPayload {
	if snapshot == nil || strings.TrimSpace(snapshot.ID) == "" {
		return contract.AgentSoulSectionPayload{}
	}
	profile, err := snapshot.ProfileEnvelope()
	if err != nil {
		return contract.AgentSoulSectionPayload{
			Present:    true,
			Active:     false,
			Valid:      false,
			SnapshotID: strings.TrimSpace(snapshot.ID),
			Digest:     strings.TrimSpace(snapshot.Digest),
		}
	}
	return contract.AgentSoulSectionPayload{
		Enabled:          profile.Compact.Enabled,
		Present:          profile.Compact.Present,
		Active:           profile.Compact.Active,
		Valid:            profile.Valid,
		ValidationStatus: soulValidationStatus(profile.Present, profile.Active, profile.Valid),
		SnapshotID:       strings.TrimSpace(snapshot.ID),
		Digest:           firstTrimmed(profile.Compact.Digest, snapshot.Digest),
		ConfigDigest:     strings.TrimSpace(profile.ConfigProvenance.Digest),
		SourcePath:       firstTrimmed(profile.Compact.SourcePath, snapshot.SourcePath),
		Role:             strings.TrimSpace(profile.Compact.Role),
		Tone:             append([]string(nil), profile.Compact.Tone...),
		Principles:       append([]string(nil), profile.Compact.Principles...),
		Truncated:        profile.Compact.Truncated || snapshot.Truncated,
		MaxBytes:         profile.Compact.MaxBytes,
		MaxBodyBytes:     profile.Compact.MaxBodyBytes,
	}
}

func soulValidationStatus(present bool, active bool, valid bool) contract.AuthoredValidationStatus {
	switch {
	case !present:
		return contract.AuthoredValidationMissing
	case !valid:
		return contract.AuthoredValidationInvalid
	case !active:
		return contract.AuthoredValidationInactive
	default:
		return contract.AuthoredValidationValid
	}
}

func workspacePayload(
	workspaceSnapshot *workspacepkg.ResolvedWorkspace,
	workspaceID string,
	rootDir string,
) contract.AgentWorkspacePayload {
	if workspaceSnapshot != nil {
		return contract.AgentWorkspacePayload{
			ID:      strings.TrimSpace(workspaceSnapshot.ID),
			Name:    strings.TrimSpace(workspaceSnapshot.Name),
			RootDir: strings.TrimSpace(workspaceSnapshot.RootDir),
		}
	}
	return contract.AgentWorkspacePayload{
		ID:      strings.TrimSpace(workspaceID),
		RootDir: strings.TrimSpace(rootDir),
	}
}

func workspaceID(workspace *workspacepkg.ResolvedWorkspace) string {
	if workspace == nil {
		return ""
	}
	return strings.TrimSpace(workspace.ID)
}

func workspaceRoot(workspace *workspacepkg.ResolvedWorkspace) string {
	if workspace == nil {
		return ""
	}
	return strings.TrimSpace(workspace.RootDir)
}

func coordinationChannelPayload(taskRecord taskpkg.Task, run taskpkg.Run) contract.CoordinationChannelPayload {
	metadata := runMetadata(run.Metadata)
	networkSpec := run.NetworkSpecSnapshot()
	channelID := strings.TrimSpace(networkSpec.ChannelID)
	channelName := firstTrimmed(networkSpec.ChannelID, channelID)
	lastActivity := latestTime(
		run.QueuedAt,
		run.ClaimedAt,
		run.StartedAt,
		taskRecord.UpdatedAt,
	)
	return contract.NormalizeCoordinationChannelPayload(contract.CoordinationChannelPayload{
		ID:          channelID,
		DisplayName: firstTrimmed(channelName, channelID),
		Purpose:     "task_run_coordination",
		WorkspaceID: strings.TrimSpace(taskRecord.WorkspaceID),
		TaskID:      strings.TrimSpace(taskRecord.ID),
		RunID:       strings.TrimSpace(run.ID),
		WorkflowID:  firstTrimmed(metadata["workflow_id"]),
		LastActivityAt: optionalTimePtr(
			lastActivity,
		),
	})
}
