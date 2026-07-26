package extensionpkg

import (
	"context"
	"database/sql"

	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/store"
)

func (h *HostAPIHandler) requireHostAPINetworkService() (hostAPINetworkService, error) {
	if h == nil || h.network == nil {
		return nil, unavailableRPCError(errors.New("extension: network service is not configured"))
	}
	return h.network, nil
}

func (h *HostAPIHandler) requireHostAPINetworkStore() (store.NetworkConversationStore, error) {
	if h == nil || h.networkStore == nil {
		return nil, unavailableRPCError(errors.New("extension: network store is not configured"))
	}
	return h.networkStore, nil
}

func (h *HostAPIHandler) resolveHostAPIDirectPeers(
	ctx context.Context,
	service hostAPINetworkService,
	workspaceID string,
	channel string,
	sessionID string,
	peerID string,
) (network.PeerInfo, network.PeerInfo, error) {
	peers, err := service.ListPeers(ctx, workspaceID, channel)
	if err != nil {
		return network.PeerInfo{}, network.PeerInfo{}, err
	}

	var local network.PeerInfo
	localFound := false
	remote, remoteFound := hostAPINetworkFindPeer(peers, peerID)
	for _, peer := range peers {
		if !peer.Local || peer.SessionID == nil {
			continue
		}
		if strings.TrimSpace(*peer.SessionID) != sessionID {
			continue
		}
		local = peer
		localFound = true
		break
	}
	if !localFound {
		return network.PeerInfo{}, network.PeerInfo{}, fmt.Errorf(
			"%w: session=%q channel=%q",
			network.ErrLocalPeerNotFound,
			sessionID,
			channel,
		)
	}
	if !remoteFound {
		return network.PeerInfo{}, network.PeerInfo{}, fmt.Errorf(
			"%w: peer_id=%q channel=%q",
			network.ErrTargetPeerNotFound,
			peerID,
			channel,
		)
	}
	if remote.SessionID == nil || strings.TrimSpace(*remote.SessionID) == "" {
		return network.PeerInfo{}, network.PeerInfo{}, fmt.Errorf(
			"%w: peer_id=%q has no participating session",
			network.ErrTargetPeerNotFound,
			peerID,
		)
	}
	return local, remote, nil
}

func hostAPINetworkFindPeer(peers []network.PeerInfo, peerID string) (network.PeerInfo, bool) {
	wanted := strings.TrimSpace(peerID)
	for _, peer := range peers {
		if strings.TrimSpace(peer.PeerID) == wanted {
			return peer, true
		}
	}
	return network.PeerInfo{}, false
}

func (h *HostAPIHandler) hostAPINetworkWorkspaceID(ctx context.Context, raw string) (string, error) {
	return h.resolveRequiredWorkspaceID(ctx, raw)
}

func mapHostAPINetworkRPCError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, network.ErrLocalPeerNotFound),
		errors.Is(err, network.ErrTargetPeerNotFound),
		errors.Is(err, store.ErrNetworkConversationNotFound),
		errors.Is(err, sql.ErrNoRows):
		return notFoundRPCError("network", "", err)
	case errors.Is(err, store.ErrNetworkDirectRoomCollision),
		errors.Is(err, store.ErrNetworkWorkContainerMismatch),
		errors.Is(err, store.ErrNetworkWorkClosed):
		return hostAPIStatusRPCError(409, "Conflict", map[string]string{extensionStateError: err.Error()})
	case errors.Is(err, network.ErrMissingField),
		errors.Is(err, store.ErrNetworkCursorInvalid),
		errors.Is(err, network.ErrInvalidField),
		errors.Is(err, network.ErrInvalidKind),
		errors.Is(err, network.ErrInvalidBody),
		errors.Is(err, network.ErrEnvelopeTooLarge),
		errors.Is(err, network.ErrExpired),
		errors.Is(err, network.ErrReplayTooOld),
		errors.Is(err, network.ErrLegacyFieldRejected):
		return invalidParamsRPCError(err)
	default:
		return err
	}
}
