package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/network/participation"
)

// BindNetworkPeer temporarily registers an active session in a task-run-owned
// Live channel without changing the session's durable creation participation.
func (m *Manager) BindNetworkPeer(
	ctx context.Context,
	sessionID string,
	spec participation.Spec,
	ownerKey string,
) error {
	if ctx == nil {
		return errors.New("session: bind network peer context is required")
	}
	session, ok := m.Get(strings.TrimSpace(sessionID))
	if !ok || session == nil {
		return fmt.Errorf("%w: %s", ErrSessionNotFound, strings.TrimSpace(sessionID))
	}
	if err := validateSessionParticipationWorkspace(spec, session.Info().WorkspaceID); err != nil {
		return err
	}
	if spec.Mode != participation.ModeLive {
		return nil
	}
	agent := session.AgentDefinition()
	return m.joinNetworkPeerWithBinding(
		ctx,
		session,
		spec,
		strings.TrimSpace(ownerKey),
		networkPeerCapabilities(agent.Capabilities),
		true,
	)
}

// RestoreNetworkPeer returns an active session to its durable creation
// participation after task-run ownership ends.
func (m *Manager) RestoreNetworkPeer(ctx context.Context, sessionID string) error {
	if ctx == nil {
		return errors.New("session: restore network peer context is required")
	}
	session, ok := m.Get(strings.TrimSpace(sessionID))
	if !ok || session == nil {
		return fmt.Errorf("%w: %s", ErrSessionNotFound, strings.TrimSpace(sessionID))
	}
	info := session.Info()
	if info.State != StateActive {
		return fmt.Errorf("%w: %s", ErrSessionNotActive, info.ID)
	}
	if info.NetworkParticipation.Mode != participation.ModeLive {
		return m.leaveNetworkPeer(ctx, session)
	}
	agent := session.AgentDefinition()
	return m.joinNetworkPeerWithBinding(
		ctx,
		session,
		info.NetworkParticipation,
		info.NetworkOwnerKey,
		networkPeerCapabilities(agent.Capabilities),
		true,
	)
}

func (m *Manager) joinNetworkPeerWithBinding(
	ctx context.Context,
	session *Session,
	spec participation.Spec,
	ownerKey string,
	capabilities []NetworkPeerCapability,
	requireActive bool,
) error {
	if session == nil || spec.Mode != participation.ModeLive {
		return nil
	}

	session.networkPeerMu.Lock()
	defer session.networkPeerMu.Unlock()

	info := session.Info()
	if requireActive && info.State != StateActive {
		return fmt.Errorf("%w: %s", ErrSessionNotActive, info.ID)
	}
	if err := validateLiveNetworkPeerID(info.AgentName, info.ID); err != nil {
		return err
	}
	lifecycle := m.currentNetworkPeerLifecycle()
	if lifecycle == nil {
		return nil
	}
	return lifecycle.JoinChannel(
		ctx,
		newNetworkPeerJoin(
			info.ID,
			info.ProfileID,
			networkPeerID(info.AgentName, info.ID),
			info.WorkspaceID,
			firstTrimmedNonEmpty(info.Name, info.AgentName),
			spec.ChannelID,
			ownerKey,
			spec,
			capabilities,
		),
	)
}
