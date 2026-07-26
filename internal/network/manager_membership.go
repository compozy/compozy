package network

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/network/participation"
	sessionpkg "github.com/compozy/agh/internal/session"
)

const managerChannelKey = "channel"

type joinChannelRequest struct {
	sessionID            string
	peerID               string
	workspaceID          string
	displayName          string
	channel              string
	ownerKey             string
	networkParticipation participation.Spec
	capabilities         []sessionpkg.NetworkPeerCapability
}

// JoinChannel registers one daemon-local Live execution as visible presence.
func (m *Manager) JoinChannel(ctx context.Context, join sessionpkg.NetworkPeerJoin) error {
	request, err := normalizeJoinChannelRequest(ctx, m, join)
	if err != nil {
		return err
	}
	if current, ok := m.sessionSnapshot(request.sessionID); ok {
		if current.peerID == request.peerID && current.workspaceID == request.workspaceID &&
			current.channel == request.channel && current.ownerKey == request.ownerKey &&
			current.networkParticipation == request.networkParticipation {
			return nil
		}
		if err := m.LeaveChannel(ctx, request.sessionID); err != nil {
			return err
		}
	}
	card, err := localPeerCardFromJoinRequest(request)
	if err != nil {
		return err
	}
	local, err := m.peers.RegisterLocalWithCapabilityCatalog(
		request.sessionID,
		request.workspaceID,
		request.channel,
		card,
		request.capabilities,
		m.now(),
	)
	if err != nil {
		return err
	}
	runtime := &managedSession{
		sessionID: local.SessionID, peerID: local.PeerID,
		workspaceID: local.WorkspaceID, channel: local.Channel,
		ownerKey: request.ownerKey, networkParticipation: request.networkParticipation,
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		m.peers.LeaveLocal(local.SessionID)
		return context.Canceled
	}
	m.sessions[runtime.sessionID] = runtime
	m.mu.Unlock()
	m.dispatchNetworkPeerLifecycleHooks(ctx, []PeerLifecycleEvent{{
		Kind:      PeerLifecycleJoined,
		Peer:      peerInfoFromLocal(local),
		Timestamp: local.JoinedAt,
	}})
	m.logger.Info(
		"network.peer.joined",
		"session_id", local.SessionID,
		"peer_id", local.PeerID,
		"workspace_id", local.WorkspaceID,
		managerChannelKey, local.Channel,
	)
	return nil
}

func normalizeJoinChannelRequest(
	ctx context.Context,
	manager *Manager,
	join sessionpkg.NetworkPeerJoin,
) (joinChannelRequest, error) {
	if ctx == nil {
		return joinChannelRequest{}, errors.New("network: join context is required")
	}
	if manager == nil {
		return joinChannelRequest{}, errors.New("network: manager is required")
	}
	if err := ctx.Err(); err != nil {
		return joinChannelRequest{}, err
	}
	if err := manager.lifecycleCtx.Err(); err != nil {
		return joinChannelRequest{}, err
	}
	request := joinChannelRequest{
		sessionID: strings.TrimSpace(join.SessionID), peerID: strings.TrimSpace(join.PeerID),
		workspaceID: strings.TrimSpace(join.WorkspaceID), displayName: strings.TrimSpace(join.DisplayName),
		channel: strings.TrimSpace(join.Channel), ownerKey: strings.TrimSpace(join.OwnerKey),
		networkParticipation: join.NetworkParticipation,
		capabilities:         cloneNetworkPeerCapabilityCatalog(join.Capabilities),
	}
	if request.sessionID == "" {
		return joinChannelRequest{}, fmt.Errorf("%w: session id is required", ErrMissingField)
	}
	if request.peerID == "" {
		return joinChannelRequest{}, fmt.Errorf("%w: peer id is required", ErrMissingField)
	}
	if err := ValidateWorkspaceID(request.workspaceID); err != nil {
		return joinChannelRequest{}, err
	}
	if err := ValidateChannel(request.channel); err != nil {
		return joinChannelRequest{}, err
	}
	if request.ownerKey == "" {
		return joinChannelRequest{}, fmt.Errorf("%w: network owner key is required", ErrMissingField)
	}
	if err := participation.ValidateSpec(request.networkParticipation); err != nil {
		return joinChannelRequest{}, fmt.Errorf("network: validate joined participation: %w", err)
	}
	if request.networkParticipation.Mode != participation.ModeLive ||
		request.networkParticipation.WorkspaceID != request.workspaceID ||
		request.networkParticipation.ChannelID != request.channel {
		return joinChannelRequest{}, fmt.Errorf(
			"%w: joined participation does not match workspace and channel",
			ErrInvalidField,
		)
	}
	return request, nil
}

func localPeerCardFromJoinRequest(request joinChannelRequest) (PeerCard, error) {
	card, err := DefaultPeerCard(request.peerID)
	if err != nil {
		return PeerCard{}, err
	}
	if request.displayName != "" {
		card.DisplayName = &request.displayName
	}
	if err := applyCapabilityBriefProjection(&card, request.capabilities); err != nil {
		return PeerCard{}, err
	}
	card.ArtifactsSupported = []string{string(KindCapability)}
	return card, nil
}

// LeaveChannel removes one daemon-local session from live membership.
func (m *Manager) LeaveChannel(ctx context.Context, sessionID string) error {
	if ctx == nil {
		return errors.New("network: leave context is required")
	}
	if m == nil {
		return errors.New("network: manager is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return fmt.Errorf("%w: session id is required", ErrMissingField)
	}
	runtime, ok := m.removeSessionRuntime(target)
	left, leftOK := m.router.Leave(target)
	if !ok || !leftOK {
		return nil
	}
	leftAt := m.now().UTC()
	m.dispatchNetworkPeerLifecycleHooks(ctx, []PeerLifecycleEvent{{
		Kind:      PeerLifecycleLeft,
		Peer:      peerInfoFromLocal(left),
		Timestamp: leftAt,
	}})
	m.logger.Info(
		"network.peer.left",
		"session_id", runtime.sessionID,
		"peer_id", runtime.peerID,
		"workspace_id", runtime.workspaceID,
		managerChannelKey, runtime.channel,
	)
	return nil
}

// ListPeers returns the current daemon-local peer snapshot.
func (m *Manager) ListPeers(ctx context.Context, workspaceID string, channel string) ([]PeerInfo, error) {
	if ctx == nil {
		return nil, errors.New("network: list peers context is required")
	}
	if m == nil || m.peers == nil {
		return nil, errors.New("network: peer registry is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return m.peers.ListPeers(strings.TrimSpace(workspaceID), strings.TrimSpace(channel)), nil
}

// ListChannels returns active in-process channels.
func (m *Manager) ListChannels(ctx context.Context, workspaceID string) ([]ChannelInfo, error) {
	if ctx == nil {
		return nil, errors.New("network: list channels context is required")
	}
	if m == nil || m.peers == nil {
		return nil, errors.New("network: peer registry is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return m.peers.ListChannels(strings.TrimSpace(workspaceID)), nil
}
