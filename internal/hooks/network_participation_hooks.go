package hooks

import (
	"context"

	"github.com/compozy/agh/internal/network/participation"
)

// NetworkParticipationPreResolvePayload is dispatched before a participation snapshot persists.
type NetworkParticipationPreResolvePayload struct {
	WorkspaceID string                 `json:"workspace_id"`
	Owner       participation.OwnerRef `json:"owner"`
	Request     *participation.Request `json:"request,omitempty"`
	Source      participation.Source   `json:"source,omitempty"`
	OwnerKey    string                 `json:"owner_key,omitempty"`
}

// NetworkParticipationPreResolvePatch may deny or narrow the request.
type NetworkParticipationPreResolvePatch struct {
	ControlPatch
	Request *participation.Request `json:"request,omitempty"`
}

// NetworkParticipationResolvedPayload is dispatched after the owning record commits.
type NetworkParticipationResolvedPayload struct {
	WorkspaceID string                 `json:"workspace_id"`
	Owner       participation.OwnerRef `json:"owner"`
	OwnerKey    string                 `json:"owner_key,omitempty"`
	Spec        participation.Spec     `json:"resolved_network_participation"`
}

// NetworkParticipationResolvedPatch is empty — resolved is observation-only.
type NetworkParticipationResolvedPatch struct{}

// DispatchNetworkParticipationPreResolve runs network.participation.pre_resolve.
func (h *Hooks) DispatchNetworkParticipationPreResolve(
	ctx context.Context,
	payload NetworkParticipationPreResolvePayload,
) (NetworkParticipationPreResolvePayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookNetworkParticipationPreResolve,
		payload,
		dispatchConfig[NetworkParticipationPreResolvePayload, NetworkParticipationPreResolvePatch]{
			match: matchNetworkParticipationPreResolve,
			apply: applyNetworkParticipationPreResolve,
			guard: guardNetworkParticipationPreResolvePatch,
			denied: func(patch NetworkParticipationPreResolvePatch) bool {
				return patch.Deny
			},
			denyErr: func(_ NetworkParticipationPreResolvePayload, report dispatchReport) error {
				if report.DenyReason == "" {
					return hookDeniedError(HookNetworkParticipationPreResolve, "network participation denied")
				}
				return hookDeniedError(HookNetworkParticipationPreResolve, report.DenyReason)
			},
		},
	)
}

// DispatchNetworkParticipationResolved runs network.participation.resolved (read-only).
func (h *Hooks) DispatchNetworkParticipationResolved(
	ctx context.Context,
	payload NetworkParticipationResolvedPayload,
) (NetworkParticipationResolvedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookNetworkParticipationResolved,
		payload,
		dispatchConfig[NetworkParticipationResolvedPayload, NetworkParticipationResolvedPatch]{
			match: matchNetworkParticipationResolved,
			apply: applyNoop[NetworkParticipationResolvedPayload, NetworkParticipationResolvedPatch],
		},
	)
}

func matchNetworkParticipationPreResolve(
	matcher HookMatcher,
	payload NetworkParticipationPreResolvePayload,
) bool {
	mode, channel := participationRequestMatcherValues(payload.Request)
	network := matcher.network()
	return matchStringField(matcher.WorkspaceID, payload.WorkspaceID) &&
		matchStringField(network.Channel, channel) &&
		matchStringField(network.ParticipationMode, mode) &&
		matchStringField(network.ParticipationSource, string(payload.Source))
}

func matchNetworkParticipationResolved(
	matcher HookMatcher,
	payload NetworkParticipationResolvedPayload,
) bool {
	network := matcher.network()
	return matchStringField(matcher.WorkspaceID, payload.WorkspaceID) &&
		matchStringField(network.Channel, payload.Spec.ChannelID) &&
		matchStringField(network.ParticipationMode, string(payload.Spec.Mode)) &&
		matchStringField(network.ParticipationSource, string(payload.Spec.Source))
}

func applyNetworkParticipationPreResolve(
	payload NetworkParticipationPreResolvePayload,
	patch NetworkParticipationPreResolvePatch,
) NetworkParticipationPreResolvePayload {
	if patch.Request != nil {
		payload.Request = participation.CloneRequest(patch.Request)
	}
	return payload
}

func participationRequestMatcherValues(request *participation.Request) (string, string) {
	if request == nil {
		return "", ""
	}
	mode := ""
	if request.Mode != nil {
		mode = string(*request.Mode)
	}
	channel := ""
	if request.ChannelID != nil {
		channel = *request.ChannelID
	}
	return mode, channel
}
