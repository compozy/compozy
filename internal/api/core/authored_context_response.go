package core

import (
	"errors"

	"strings"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/soul"
)

func soulMutationResponse(
	target authoredAgentTarget,
	result *soul.MutationResult,
) (contract.AgentSoulMutationResponse, error) {
	if result == nil {
		return contract.AgentSoulMutationResponse{}, errors.New("soul mutation result is required")
	}
	payload := contract.AgentSoulPayloadFromResolved(
		target.agentName,
		&result.Soul,
		result.Snapshot.ID,
		target.soulConfigProvenance(),
	)
	payload.RevisionID = result.Revision.ID
	revision, err := contract.AgentSoulRevisionPayloadFromDomain(result.Revision)
	if err != nil {
		return contract.AgentSoulMutationResponse{}, err
	}
	return contract.AgentSoulMutationResponse{Soul: payload, Revision: revision}, nil
}

func soulHistoryResponse(result soul.HistoryResult) (contract.AgentSoulHistoryResponse, error) {
	revisions := make([]contract.AgentSoulRevisionPayload, 0, len(result.Revisions))
	for _, revision := range result.Revisions {
		converted, err := contract.AgentSoulRevisionPayloadFromDomain(revision)
		if err != nil {
			return contract.AgentSoulHistoryResponse{}, err
		}
		revisions = append(revisions, converted)
	}
	return contract.AgentSoulHistoryResponse{Revisions: revisions}, nil
}

func heartbeatMutationResponse(result *heartbeat.MutationResult) (contract.HeartbeatMutationResponse, error) {
	if result == nil {
		return contract.HeartbeatMutationResponse{}, errors.New("heartbeat mutation result is required")
	}
	policy, err := contract.HeartbeatPolicyPayloadFromResolved(
		result.Revision.AgentName,
		&result.Policy,
		result.Snapshot.ID,
	)
	if err != nil {
		return contract.HeartbeatMutationResponse{}, err
	}
	revision, err := contract.HeartbeatRevisionPayloadFromDomain(result.Revision)
	if err != nil {
		return contract.HeartbeatMutationResponse{}, err
	}
	return contract.HeartbeatMutationResponse{Heartbeat: policy, Revision: revision}, nil
}

func heartbeatHistoryResponse(result heartbeat.HistoryResult) (contract.HeartbeatHistoryResponse, error) {
	revisions := make([]contract.HeartbeatRevisionPayload, 0, len(result.Revisions))
	for _, revision := range result.Revisions {
		converted, err := contract.HeartbeatRevisionPayloadFromDomain(revision)
		if err != nil {
			return contract.HeartbeatHistoryResponse{}, err
		}
		revisions = append(revisions, converted)
	}
	return contract.HeartbeatHistoryResponse{Revisions: revisions}, nil
}

func agentSoulPayloadFromRefreshResult(result session.SoulRefreshResult) (contract.AgentSoulPayload, error) {
	if result.Soul != nil {
		snapshotID := ""
		provenance := result.ConfigProvenance
		if result.Snapshot != nil {
			snapshotID = result.Snapshot.ID
			envelope, err := result.Snapshot.ProfileEnvelope()
			if err == nil && strings.TrimSpace(provenance.Digest) == "" {
				provenance = envelope.ConfigProvenance
			}
		}
		return contract.AgentSoulPayloadFromResolved(result.AgentName, result.Soul, snapshotID, provenance), nil
	}
	if result.Snapshot == nil {
		return contract.AgentSoulPayload{}, nil
	}
	return agentSoulPayloadFromSnapshot(*result.Snapshot)
}

func agentSoulPayloadFromSnapshot(snapshot soul.Snapshot) (contract.AgentSoulPayload, error) {
	envelope, err := snapshot.ProfileEnvelope()
	if err != nil {
		return contract.AgentSoulPayload{}, err
	}
	resolved := soul.ResolvedSoul{
		Enabled:     envelope.ReadModel.Enabled,
		Present:     envelope.Present,
		Active:      envelope.Active,
		Valid:       envelope.Valid,
		SourcePath:  snapshot.SourcePath,
		Digest:      snapshot.Digest,
		Profile:     envelope.Profile,
		Compact:     envelope.Compact,
		ReadModel:   envelope.ReadModel,
		Diagnostics: envelope.Diagnostics,
	}
	return contract.AgentSoulPayloadFromResolved(
		snapshot.AgentName,
		&resolved,
		snapshot.ID,
		envelope.ConfigProvenance,
	), nil
}
