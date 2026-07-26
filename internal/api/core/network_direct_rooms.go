package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/store"
	"github.com/gin-gonic/gin"
)

// ResolveNetworkDirectRoom creates or returns a deterministic two-party direct room.
func (h *BaseHandlers) ResolveNetworkDirectRoom(c *gin.Context) {
	service, ok := h.requireNetworkReadDependencies(c)
	if !ok {
		return
	}
	channel, err := normalizeNetworkChannel(c.Param("channel"))
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	scope, ok := h.resolveWorkspaceScope(c)
	if !ok {
		return
	}

	var req contract.NetworkDirectResolveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode network direct resolve request: %w", h.transportName(), err),
		)
		return
	}
	sessionID := strings.TrimSpace(req.SessionID)
	peerID := strings.TrimSpace(req.PeerID)
	if sessionID == "" {
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(errors.New("session_id is required")))
		return
	}
	if err := network.ValidatePeerID(peerID); err != nil {
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(err))
		return
	}

	localPeer, remotePeer, err := h.resolveDirectRoomPeers(
		c.Request.Context(),
		service,
		scope.NetworkWorkspaceID(),
		channel,
		sessionID,
		peerID,
	)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	networkWorkspaceID := scope.NetworkWorkspaceID()
	directID, sessionA, sessionB, err := store.NetworkDirectRoomIdentity(
		networkWorkspaceID,
		channel,
		*localPeer.SessionID,
		*remotePeer.SessionID,
	)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	now := h.nowUTC()
	direct, err := h.NetworkStore.ResolveDirectRoom(c.Request.Context(), store.NetworkDirectRoomEntry{
		WorkspaceID:    networkWorkspaceID,
		Channel:        channel,
		DirectID:       directID,
		SessionA:       sessionA,
		SessionB:       sessionB,
		OpenedAt:       now,
		LastActivityAt: now,
	})
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.NetworkDirectRoomResponse{
		Direct: NetworkDirectRoomPayloadFromStore(direct),
	})
}

// NetworkDirectRoom returns one direct-room summary.
func (h *BaseHandlers) NetworkDirectRoom(c *gin.Context) {
	if _, ok := h.requireNetworkReadDependencies(c); !ok {
		return
	}
	channel, err := normalizeNetworkChannel(c.Param("channel"))
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	scope, ok := h.resolveWorkspaceScope(c)
	if !ok {
		return
	}
	directID := strings.TrimSpace(c.Param("direct_id"))
	if err := network.ValidateConversationID(directID, "direct_id"); err != nil {
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(err))
		return
	}

	direct, err := h.NetworkStore.GetDirectRoom(c.Request.Context(), scope.NetworkChannelRef(channel), directID)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.NetworkDirectRoomResponse{
		Direct: NetworkDirectRoomPayloadFromStore(direct),
	})
}

func (h *BaseHandlers) resolveDirectRoomPeers(
	ctx context.Context,
	service NetworkService,
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
	remote, remoteFound := findPeerInfo(peers, peerID)
	for _, peer := range peers {
		if peer.SessionID == nil || strings.TrimSpace(*peer.SessionID) != sessionID {
			continue
		}
		if !peer.Local {
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
	if strings.TrimSpace(*local.SessionID) == strings.TrimSpace(*remote.SessionID) {
		return network.PeerInfo{}, network.PeerInfo{}, fmt.Errorf(
			"%w: direct room sessions must differ",
			network.ErrInvalidField,
		)
	}
	return local, remote, nil
}
