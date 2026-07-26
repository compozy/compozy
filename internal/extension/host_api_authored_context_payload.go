package extensionpkg

import (
	"errors"

	"strings"

	apicontract "github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/heartbeat"

	"github.com/compozy/agh/internal/soul"
)

func hostAPISoulPayload(
	target hostAPIAuthoredAgentTarget,
	resolved *soul.ResolvedSoul,
	snapshotID string,
	err error,
) (apicontract.AgentSoulPayload, error) {
	if err != nil && !errors.Is(err, soul.ErrInvalid) {
		return apicontract.AgentSoulPayload{}, mapHostAPISoulRPCError(err)
	}
	payload := apicontract.AgentSoulPayloadFromResolved(
		target.agentName,
		resolved,
		strings.TrimSpace(snapshotID),
		target.soulConfigProvenance(),
	)
	if err := apicontract.ValidateAuthoredContextRedacted(payload); err != nil {
		return apicontract.AgentSoulPayload{}, mapHostAPISoulRPCError(err)
	}
	return payload, nil
}

func hostAPIHeartbeatPayload(
	target hostAPIAuthoredAgentTarget,
	policy *heartbeat.ResolvedPolicy,
	snapshotID string,
	err error,
) (apicontract.HeartbeatPolicyPayload, error) {
	if err != nil && !errors.Is(err, heartbeat.ErrInvalid) {
		return apicontract.HeartbeatPolicyPayload{}, mapHostAPIHeartbeatRPCError(err)
	}
	payload, err := apicontract.HeartbeatPolicyPayloadFromResolved(
		target.agentName,
		policy,
		strings.TrimSpace(snapshotID),
	)
	if err != nil {
		return apicontract.HeartbeatPolicyPayload{}, mapHostAPIHeartbeatRPCError(err)
	}
	if err := apicontract.ValidateAuthoredContextRedacted(payload); err != nil {
		return apicontract.HeartbeatPolicyPayload{}, mapHostAPIHeartbeatRPCError(err)
	}
	return payload, nil
}

func hostAPISoulMutationResponse(
	target hostAPIAuthoredAgentTarget,
	result *soul.MutationResult,
) (apicontract.AgentSoulMutationResponse, error) {
	if result == nil {
		return apicontract.AgentSoulMutationResponse{}, mapHostAPISoulRPCError(errors.New("soul result is required"))
	}
	payload := apicontract.AgentSoulPayloadFromResolved(
		target.agentName,
		&result.Soul,
		result.Snapshot.ID,
		target.soulConfigProvenance(),
	)
	payload.RevisionID = result.Revision.ID
	revision, err := apicontract.AgentSoulRevisionPayloadFromDomain(result.Revision)
	if err != nil {
		return apicontract.AgentSoulMutationResponse{}, mapHostAPISoulRPCError(err)
	}
	response := apicontract.AgentSoulMutationResponse{Soul: payload, Revision: revision}
	if err := apicontract.ValidateAuthoredContextRedacted(response); err != nil {
		return apicontract.AgentSoulMutationResponse{}, mapHostAPISoulRPCError(err)
	}
	return response, nil
}

func hostAPISoulHistoryResponse(result soul.HistoryResult) (apicontract.AgentSoulHistoryResponse, error) {
	revisions := make([]apicontract.AgentSoulRevisionPayload, 0, len(result.Revisions))
	for _, revision := range result.Revisions {
		converted, err := apicontract.AgentSoulRevisionPayloadFromDomain(revision)
		if err != nil {
			return apicontract.AgentSoulHistoryResponse{}, mapHostAPISoulRPCError(err)
		}
		revisions = append(revisions, converted)
	}
	response := apicontract.AgentSoulHistoryResponse{Revisions: revisions}
	if err := apicontract.ValidateAuthoredContextRedacted(response); err != nil {
		return apicontract.AgentSoulHistoryResponse{}, mapHostAPISoulRPCError(err)
	}
	return response, nil
}

func hostAPIHeartbeatMutationResponse(
	result *heartbeat.MutationResult,
) (apicontract.HeartbeatMutationResponse, error) {
	if result == nil {
		return apicontract.HeartbeatMutationResponse{}, mapHostAPIHeartbeatRPCError(
			errors.New("heartbeat result is required"),
		)
	}
	policy, err := apicontract.HeartbeatPolicyPayloadFromResolved(
		result.Revision.AgentName,
		&result.Policy,
		result.Snapshot.ID,
	)
	if err != nil {
		return apicontract.HeartbeatMutationResponse{}, mapHostAPIHeartbeatRPCError(err)
	}
	revision, err := apicontract.HeartbeatRevisionPayloadFromDomain(result.Revision)
	if err != nil {
		return apicontract.HeartbeatMutationResponse{}, mapHostAPIHeartbeatRPCError(err)
	}
	response := apicontract.HeartbeatMutationResponse{Heartbeat: policy, Revision: revision}
	if err := apicontract.ValidateAuthoredContextRedacted(response); err != nil {
		return apicontract.HeartbeatMutationResponse{}, mapHostAPIHeartbeatRPCError(err)
	}
	return response, nil
}

func hostAPIHeartbeatHistoryResponse(result heartbeat.HistoryResult) (apicontract.HeartbeatHistoryResponse, error) {
	revisions := make([]apicontract.HeartbeatRevisionPayload, 0, len(result.Revisions))
	for _, revision := range result.Revisions {
		converted, err := apicontract.HeartbeatRevisionPayloadFromDomain(revision)
		if err != nil {
			return apicontract.HeartbeatHistoryResponse{}, mapHostAPIHeartbeatRPCError(err)
		}
		revisions = append(revisions, converted)
	}
	response := apicontract.HeartbeatHistoryResponse{Revisions: revisions}
	if err := apicontract.ValidateAuthoredContextRedacted(response); err != nil {
		return apicontract.HeartbeatHistoryResponse{}, mapHostAPIHeartbeatRPCError(err)
	}
	return response, nil
}
