package core

import (
	"context"

	"errors"
	"fmt"
	"net/http"

	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/session"
	"github.com/gin-gonic/gin"
)

const (
	apiCapabilityBriefExtKey   = "agh.capabilities_brief"
	apiCapabilityCatalogExtKey = "agh.capability_catalog"
)

func (h *BaseHandlers) networkServiceRequired() (NetworkService, error) {
	if !h.Config.Network.Enabled {
		return nil, errors.New("api: network is disabled")
	}
	if h.Network == nil {
		return nil, errors.New("api: network service is required when network is enabled")
	}
	return h.Network, nil
}

// NetworkStatus returns the current daemon-owned network runtime status.
func (h *BaseHandlers) NetworkStatus(c *gin.Context) {
	payload, err := h.networkStatusPayload(c.Request.Context())
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, contract.NetworkStatusResponse{Network: *payload})
}

// NetworkPeers returns the current visible peers, optionally filtered by channel.
func (h *BaseHandlers) NetworkPeers(c *gin.Context) {
	service, err := h.networkServiceRequired()
	if err != nil {
		h.respondError(c, http.StatusServiceUnavailable, err)
		return
	}
	scope, ok := h.resolveWorkspaceScope(c)
	if !ok {
		return
	}

	peers, err := service.ListPeers(
		c.Request.Context(),
		scope.NetworkWorkspaceID(),
		strings.TrimSpace(c.Query("channel")),
	)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	sessionByID := h.networkPeerSessionInfoMap(c.Request.Context(), peers)
	payload := make([]contract.NetworkPeerPayload, 0, len(peers))
	for _, peer := range peers {
		payload = append(payload, networkPeerPayloadFromInfoWithSessions(peer, sessionByID))
	}
	sortNetworkPeerPayloads(payload)
	c.JSON(http.StatusOK, contract.NetworkPeersResponse{Peers: payload})
}

func (h *BaseHandlers) networkPeerSessionInfoMap(
	ctx context.Context,
	peers []network.PeerInfo,
) map[string]*session.Info {
	if h == nil || h.Sessions == nil || len(peers) == 0 {
		return nil
	}

	wanted := make(map[string]struct{}, len(peers))
	for _, peer := range peers {
		if peer.SessionID == nil {
			continue
		}

		sessionID := strings.TrimSpace(*peer.SessionID)
		if sessionID == "" {
			continue
		}
		wanted[sessionID] = struct{}{}
	}
	if len(wanted) == 0 {
		return nil
	}

	infos, err := h.Sessions.ListAll(ctx)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Warn(
				h.transportName()+": skip network peer session enrichment",
				"error",
				err,
			)
		}
		return nil
	}

	sessionByID := make(map[string]*session.Info, len(wanted))
	for _, info := range infos {
		if info == nil {
			continue
		}
		sessionID := strings.TrimSpace(info.ID)
		if _, ok := wanted[sessionID]; ok {
			sessionByID[sessionID] = info
		}
	}
	if len(sessionByID) == 0 {
		return nil
	}
	return sessionByID
}

// NetworkSend validates and forwards one outbound network send request.
func (h *BaseHandlers) NetworkSend(c *gin.Context) {
	service, err := h.networkServiceRequired()
	if err != nil {
		h.respondError(c, http.StatusServiceUnavailable, err)
		return
	}
	scope, ok := h.resolveWorkspaceScope(c)
	if !ok {
		return
	}

	var req contract.NetworkSendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode network send request: %w", h.transportName(), err),
		)
		return
	}
	if !scope.BodyWorkspaceIDMatches(req.WorkspaceID) {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewNetworkValidationError(errors.New("workspace_id does not match path")),
		)
		return
	}
	req.WorkspaceID = scope.NetworkWorkspaceID()
	if strings.TrimSpace(req.SessionID) == "" {
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(errors.New("session_id is required")))
		return
	}
	if _, err := h.requireSessionInWorkspace(
		c.Request.Context(),
		scope.SessionWorkspaceID(),
		req.SessionID,
	); err != nil {
		h.respondError(c, statusForWorkspaceScopedResourceError(err), err)
		return
	}
	if err := h.populateNetworkDirectSendTarget(c.Request.Context(), service, &scope, &req); err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}

	sendReq, err := NetworkSendRequestFromPayload(req)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}

	id, err := service.Send(c.Request.Context(), sendReq)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.NetworkSendResponse{Message: NetworkSendPayloadFromRequest(id, req)})
}

func (h *BaseHandlers) populateNetworkDirectSendTarget(
	ctx context.Context,
	service NetworkService,
	scope *workspaceScope,
	req *contract.NetworkSendRequest,
) error {
	if req == nil ||
		strings.TrimSpace(req.Surface) != string(network.SurfaceDirect) ||
		strings.TrimSpace(req.To) != "" ||
		strings.TrimSpace(req.DirectID) == "" {
		return nil
	}
	if h == nil || h.NetworkStore == nil {
		return errors.New("api: network store is required to resolve direct send target")
	}
	channel, err := normalizeNetworkChannel(req.Channel)
	if err != nil {
		return err
	}
	directID := strings.TrimSpace(req.DirectID)
	if err := network.ValidateConversationID(directID, "direct_id"); err != nil {
		return err
	}
	direct, err := h.NetworkStore.GetDirectRoom(ctx, scope.NetworkChannelRef(channel), directID)
	if err != nil {
		return err
	}
	sessionID := strings.TrimSpace(req.SessionID)
	targetSessionID := ""
	switch sessionID {
	case strings.TrimSpace(direct.SessionA):
		targetSessionID = strings.TrimSpace(direct.SessionB)
	case strings.TrimSpace(direct.SessionB):
		targetSessionID = strings.TrimSpace(direct.SessionA)
	default:
		return fmt.Errorf(
			"%w: session %q is not part of direct room %q",
			network.ErrInvalidField,
			sessionID,
			directID,
		)
	}
	peers, err := service.ListPeers(ctx, scope.NetworkWorkspaceID(), "")
	if err != nil {
		return err
	}
	for _, peer := range peers {
		if peer.SessionID == nil || strings.TrimSpace(*peer.SessionID) != targetSessionID {
			continue
		}
		req.To = strings.TrimSpace(peer.PeerID)
		return nil
	}
	return fmt.Errorf(
		"%w: session=%q channel=%q",
		network.ErrTargetPeerNotFound,
		targetSessionID,
		channel,
	)
}

// NetworkInbox returns the queued inbound envelopes for one local session.
func (h *BaseHandlers) NetworkInbox(c *gin.Context) {
	service, err := h.networkServiceRequired()
	if err != nil {
		h.respondError(c, http.StatusServiceUnavailable, err)
		return
	}
	scope, ok := h.resolveWorkspaceScope(c)
	if !ok {
		return
	}

	sessionID := strings.TrimSpace(c.Query("session_id"))
	if sessionID == "" {
		sessionID = strings.TrimSpace(c.Query("session"))
	}
	if sessionID == "" {
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(errors.New("session_id query is required")))
		return
	}
	if _, err := h.requireSessionInWorkspace(c.Request.Context(), scope.SessionWorkspaceID(), sessionID); err != nil {
		h.respondError(c, statusForWorkspaceScopedResourceError(err), err)
		return
	}

	messages, err := service.Inbox(c.Request.Context(), sessionID)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.NetworkInboxResponse{Messages: NetworkEnvelopePayloadsFromEnvelopes(messages)})
}
