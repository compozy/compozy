package extensionpkg

import (
	"context"

	"errors"
	"fmt"

	"strings"

	apicontract "github.com/compozy/agh/internal/api/contract"

	extensioncontract "github.com/compozy/agh/internal/extension/contract"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/soul"
)

func (h *HostAPIHandler) hostAPISessionHealthPayload(
	ctx context.Context,
	workspaceRef string,
	sessionID string,
) (apicontract.SessionHealthPayload, error) {
	if h.sessionHealth == nil {
		return apicontract.SessionHealthPayload{}, unavailableRPCError(
			errors.New("extension: session health service is not configured"),
		)
	}
	id := strings.TrimSpace(sessionID)
	if id == "" {
		return apicontract.SessionHealthPayload{}, invalidParamsRPCError(errors.New("session_id is required"))
	}
	info, err := h.requireHostAPISessionWorkspace(ctx, workspaceRef, id)
	if err != nil {
		return apicontract.SessionHealthPayload{}, err
	}
	health, err := h.sessionHealth.GetSessionHealth(ctx, id)
	if err != nil {
		return apicontract.SessionHealthPayload{}, mapHostAPIHeartbeatRPCError(err)
	}
	payload, err := apicontract.SessionHealthPayloadFromDomain(health)
	if err != nil {
		return apicontract.SessionHealthPayload{}, mapHostAPIHeartbeatRPCError(err)
	}
	if err := apicontract.ValidateAuthoredContextRedacted(payload); err != nil {
		return apicontract.SessionHealthPayload{}, mapHostAPIHeartbeatRPCError(err)
	}
	if strings.TrimSpace(payload.WorkspaceID) != strings.TrimSpace(info.WorkspaceID) {
		return apicontract.SessionHealthPayload{}, mapHostAPIHeartbeatRPCError(
			fmt.Errorf(
				"%w: session=%q workspace_id=%q",
				session.ErrSessionNotFound,
				id,
				strings.TrimSpace(info.WorkspaceID),
			),
		)
	}
	return payload, nil
}

func (h *HostAPIHandler) hostAPIWakeEvents(
	ctx context.Context,
	target hostAPIAuthoredAgentTarget,
	sessionID string,
) ([]apicontract.HeartbeatWakeEventPayload, error) {
	if h.wakeEvents == nil {
		return nil, nil
	}
	events, err := h.wakeEvents.ListHeartbeatWakeEvents(ctx, heartbeat.WakeEventListQuery{
		WorkspaceID: target.workspaceID,
		AgentName:   target.agentName,
		SessionID:   strings.TrimSpace(sessionID),
		Limit:       defaultHostAPIWakeEventLimit,
	})
	if err != nil {
		return nil, err
	}
	payload := make([]apicontract.HeartbeatWakeEventPayload, 0, len(events))
	for _, event := range events {
		converted, err := apicontract.HeartbeatWakeEventPayloadFromDomain(event)
		if err != nil {
			return nil, err
		}
		payload = append(payload, converted)
	}
	return payload, apicontract.ValidateAuthoredContextRedacted(payload)
}

func hostAPIAgentSoulPayloadFromRefreshResult(
	result session.SoulRefreshResult,
) (apicontract.AgentSoulPayload, error) {
	if result.Soul != nil {
		snapshotID := ""
		provenance := result.ConfigProvenance
		if result.Snapshot != nil {
			snapshotID = strings.TrimSpace(result.Snapshot.ID)
			envelope, err := result.Snapshot.ProfileEnvelope()
			if err == nil && strings.TrimSpace(provenance.Digest) == "" {
				provenance = envelope.ConfigProvenance
			}
		}
		payload := apicontract.AgentSoulPayloadFromResolved(result.AgentName, result.Soul, snapshotID, provenance)
		return payload, apicontract.ValidateAuthoredContextRedacted(payload)
	}
	if result.Snapshot == nil {
		return apicontract.AgentSoulPayload{}, nil
	}
	payload, err := hostAPIAgentSoulPayloadFromSnapshot(*result.Snapshot)
	if err != nil {
		return apicontract.AgentSoulPayload{}, err
	}
	return payload, apicontract.ValidateAuthoredContextRedacted(payload)
}

func hostAPIAgentSoulPayloadFromSnapshot(snapshot soul.Snapshot) (apicontract.AgentSoulPayload, error) {
	envelope, err := snapshot.ProfileEnvelope()
	if err != nil {
		return apicontract.AgentSoulPayload{}, mapHostAPISoulRPCError(err)
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
	return apicontract.AgentSoulPayloadFromResolved(
		snapshot.AgentName,
		&resolved,
		snapshot.ID,
		envelope.ConfigProvenance,
	), nil
}

func hostAPISoulActor(ctx context.Context) soul.AuthoringIdentity {
	return soul.AuthoringIdentity{
		Kind: hostAPIAuthoredContextExtensionKey,
		Ref:  hostAPIExtensionNameFromContext(ctx),
	}
}

func hostAPISoulOrigin(method extensioncontract.HostAPIMethod) soul.AuthoringIdentity {
	return soul.AuthoringIdentity{Kind: hostAPIAuthoredContextHostAPIKey, Ref: strings.TrimSpace(string(method))}
}

func hostAPIHeartbeatActor(ctx context.Context) heartbeat.AuthoringIdentity {
	return heartbeat.AuthoringIdentity{
		Kind: string(heartbeat.ActorKindExtension),
		Ref:  hostAPIExtensionNameFromContext(ctx),
	}
}
