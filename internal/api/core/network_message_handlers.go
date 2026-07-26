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
	"github.com/compozy/agh/internal/store"

	"github.com/gin-gonic/gin"
)

// NetworkChannelMessages returns the read-only message timeline for one network channel.
func (h *BaseHandlers) NetworkChannelMessages(c *gin.Context) {
	service, err := h.networkServiceRequired()
	if err != nil {
		h.respondError(c, http.StatusServiceUnavailable, err)
		return
	}
	networkStore, err := h.networkStoreRequired()
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
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
	query, err := parseNetworkMessageQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}

	sessions, err := h.Sessions.ListAll(c.Request.Context())
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	networkWorkspaceID := scope.NetworkWorkspaceID()
	peers, err := service.ListPeers(c.Request.Context(), networkWorkspaceID, channel)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}

	query.WorkspaceID = networkWorkspaceID
	query.Channel = channel
	if err := query.Validate(); err != nil {
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(err))
		return
	}
	storeQuery := overfetchNetworkMessageQuery(query)
	rawMessages, messages, err := h.loadPublicChannelTimeline(c.Request.Context(), networkStore, storeQuery)
	if err != nil {
		h.respondNetworkMessageError(c, err)
		return
	}

	if err := h.ensureNetworkChannelTimelineExists(
		c.Request.Context(),
		networkStore,
		scope.NetworkChannelRef(channel),
		sessions,
		peers,
		rawMessages,
	); err != nil {
		if isNetworkChannelNotFound(err) {
			h.respondError(c, http.StatusNotFound, err)
			return
		}
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}

	response, err := h.networkChannelMessagesResponse(messages, sessions, peers, storeQuery, query)
	if err != nil {
		h.respondNetworkMessageError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *BaseHandlers) networkChannelMessagesResponse(
	messages []store.NetworkMessageEntry,
	sessions []*session.Info,
	peers []network.PeerInfo,
	storeQuery store.NetworkMessageQuery,
	requestQuery store.NetworkMessageQuery,
) (contract.NetworkChannelMessagesResponse, error) {
	payload, err := networkTimelinePayloads(
		messages,
		sessionInfoMapByID(sessions),
		peerInfoMapByID(peers),
		networkTimelinePayloadQuery(storeQuery),
	)
	if err != nil {
		return contract.NetworkChannelMessagesResponse{}, err
	}
	payload, page := trimNetworkMessagePayloadPage(
		payload,
		requestQuery.AfterMessageID,
		requestQuery.Limit,
	)
	return contract.NetworkChannelMessagesResponse{Messages: payload, Page: page}, nil
}

func (h *BaseHandlers) ensureNetworkChannelTimelineExists(
	ctx context.Context,
	networkStore NetworkStore,
	ref store.NetworkChannelRef,
	sessions []*session.Info,
	peers []network.PeerInfo,
	messages []store.NetworkMessageEntry,
) error {
	metadata, err := h.loadNetworkChannelMetadata(ctx, networkStore, ref)
	if err != nil {
		return fmt.Errorf("load network channel metadata: %w", err)
	}
	if len(messages) > 0 || networkChannelExists(sessions, peers, metadata, ref.WorkspaceID, ref.Channel) {
		return nil
	}
	hasHistory, err := networkChannelHasPersistedProjection(ctx, networkStore, ref.WorkspaceID, ref.Channel)
	if err != nil {
		return err
	}
	if hasHistory {
		return nil
	}
	return fmt.Errorf("%w: %s", errNetworkChannelNotFound, ref.Channel)
}

func (h *BaseHandlers) networkChannelMetadataForUpdate(
	ctx context.Context,
	networkStore NetworkStore,
	ref store.NetworkChannelRef,
) (store.NetworkChannelEntry, error) {
	metadata, err := h.loadNetworkChannelMetadata(ctx, networkStore, ref)
	if err != nil {
		return store.NetworkChannelEntry{}, err
	}
	if metadata != nil {
		return *metadata, nil
	}
	now := h.nowUTC()
	return store.NetworkChannelEntry{
		WorkspaceID:  strings.TrimSpace(ref.WorkspaceID),
		Channel:      strings.TrimSpace(ref.Channel),
		Purpose:      "network_channel",
		FanoutPolicy: store.NetworkFanoutPolicyCapabilityMatch,
		CreatedBy:    "api",
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// NetworkPeerMessages returns the directed message timeline for one network peer.
func (h *BaseHandlers) NetworkPeerMessages(c *gin.Context) {
	service, err := h.networkServiceRequired()
	if err != nil {
		h.respondError(c, http.StatusServiceUnavailable, err)
		return
	}
	networkStore, err := h.networkStoreRequired()
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}

	peerID := strings.TrimSpace(c.Param("peer_id"))
	if peerID == "" {
		err := NewNetworkValidationError(errors.New("peer_id path is required"))
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	query, err := parseNetworkMessageQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}

	scope, ok := h.resolveWorkspaceScope(c)
	if !ok {
		return
	}
	networkWorkspaceID := scope.NetworkWorkspaceID()
	peers, err := service.ListPeers(c.Request.Context(), networkWorkspaceID, "")
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	if _, ok := findPeerInfo(peers, peerID); !ok {
		h.respondError(c, http.StatusNotFound, fmt.Errorf("api: network peer not found: %s", peerID))
		return
	}

	sessions, err := h.Sessions.ListAll(c.Request.Context())
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}

	query.WorkspaceID = networkWorkspaceID
	query.PeerID = peerID
	query.DirectedOnly = !query.IncludePresence
	if err := query.Validate(); err != nil {
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(err))
		return
	}
	storeQuery := overfetchNetworkMessageQuery(query)
	messages, err := h.loadVisiblePeerMessages(c.Request.Context(), networkStore, storeQuery)
	if err != nil {
		h.respondNetworkMessageError(c, err)
		return
	}

	sessionByID := sessionInfoMapByID(sessions)
	peerByID := peerInfoMapByID(peers)
	payload, err := networkTimelinePayloads(
		messages,
		sessionByID,
		peerByID,
		networkTimelinePayloadQuery(storeQuery),
	)
	if err != nil {
		h.respondNetworkMessageError(c, err)
		return
	}

	payload, page := trimNetworkMessagePayloadPage(
		payload,
		query.AfterMessageID,
		query.Limit,
	)
	c.JSON(http.StatusOK, contract.NetworkPeerMessagesResponse{Messages: payload, Page: page})
}

// NetworkPeer returns one selected peer detail payload.
func (h *BaseHandlers) NetworkPeer(c *gin.Context) {
	service, err := h.networkServiceRequired()
	if err != nil {
		h.respondError(c, http.StatusServiceUnavailable, err)
		return
	}
	networkStore, err := h.networkStoreRequired()
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}

	peerID := strings.TrimSpace(c.Param("peer_id"))
	if peerID == "" {
		err := NewNetworkValidationError(errors.New("peer_id path is required"))
		h.respondError(c, http.StatusBadRequest, err)
		return
	}

	scope, ok := h.resolveWorkspaceScope(c)
	if !ok {
		return
	}
	peers, err := service.ListPeers(c.Request.Context(), scope.NetworkWorkspaceID(), "")
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	peer, ok := findPeerInfo(peers, peerID)
	if !ok {
		h.respondError(c, http.StatusNotFound, fmt.Errorf("api: network peer not found: %s", peerID))
		return
	}

	sessions, err := h.Sessions.ListAll(c.Request.Context())
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}

	auditEntries, err := h.loadPeerAuditEntries(c.Request.Context(), networkStore, peer)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}

	payload := NetworkPeerDetailPayloadFromInfo(
		peer,
		sessionInfoMapByID(sessions),
		summarizePeerMetrics(peer, auditEntries),
	)
	c.JSON(http.StatusOK, contract.NetworkPeerResponse{Peer: payload})
}
