package core

import (
	"context"

	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/store"

	"github.com/gin-gonic/gin"
)

// NetworkSubscriptions returns peer delivery preferences for a channel or thread.
func (h *BaseHandlers) NetworkSubscriptions(c *gin.Context) {
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
	limit, err := parsePositiveIntQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(err))
		return
	}
	if limit == 0 {
		limit = 100
	}
	query := store.NetworkSubscriptionQuery{
		WorkspaceID: scope.NetworkWorkspaceID(),
		Channel:     channel,
		ThreadID:    strings.TrimSpace(c.Query("thread_id")),
		SessionID:   strings.TrimSpace(c.Query("session_id")),
		Limit:       limit,
	}
	if err := query.Validate(); err != nil {
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(err))
		return
	}
	subscriptions, err := h.NetworkStore.ListNetworkSubscriptions(c.Request.Context(), query)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.NetworkSubscriptionsResponse{
		Subscriptions: NetworkSubscriptionPayloadsFromStore(subscriptions),
	})
}

// UpsertNetworkSubscription sets one peer delivery preference for a channel or thread.
func (h *BaseHandlers) UpsertNetworkSubscription(c *gin.Context) {
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
	var req contract.NetworkSubscriptionRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode network subscription request: %w", h.transportName(), err),
		)
		return
	}
	now := h.nowUTC()
	entry := store.NetworkSubscriptionEntry{
		WorkspaceID: scope.NetworkWorkspaceID(),
		Channel:     channel,
		ThreadID:    strings.TrimSpace(req.ThreadID),
		SessionID:   strings.TrimSpace(req.SessionID),
		Mode:        strings.TrimSpace(req.Mode),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := entry.Validate(); err != nil {
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(err))
		return
	}
	channelMetadata, err := h.networkChannelMetadataForUpdate(
		c.Request.Context(),
		h.NetworkStore,
		store.NetworkChannelRef{WorkspaceID: entry.WorkspaceID, Channel: entry.Channel},
	)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	if err := h.NetworkStore.PutNetworkSubscriptionWithChannel(
		c.Request.Context(),
		channelMetadata,
		entry,
	); err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.NetworkSubscriptionResponse{
		Subscription: NetworkSubscriptionPayloadFromStore(entry),
	})
}

// DeleteNetworkSubscription removes one peer delivery preference.
func (h *BaseHandlers) DeleteNetworkSubscription(c *gin.Context) {
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
	ref := store.NetworkSubscriptionRef{
		WorkspaceID: scope.NetworkWorkspaceID(),
		Channel:     channel,
		ThreadID:    strings.TrimSpace(c.Query("thread_id")),
		SessionID:   strings.TrimSpace(c.Param("session_id")),
	}
	if err := ref.Validate(); err != nil {
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(err))
		return
	}
	if err := h.NetworkStore.DeleteNetworkSubscription(c.Request.Context(), ref); err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	c.Status(http.StatusNoContent)
}

type networkThreadSessionCostStore interface {
	ListNetworkThreadSessionTokenStats(
		ctx context.Context,
		query store.NetworkThreadSessionTokenStatsQuery,
	) ([]store.NetworkThreadSessionTokenStats, error)
}

type networkThreadTaskLinkStore interface {
	ListNetworkTaskThreadOrigins(
		ctx context.Context,
		query store.NetworkTaskThreadOriginQuery,
	) ([]store.NetworkTaskThreadOrigin, error)
}

func (h *BaseHandlers) networkThreadSessionCosts(
	ctx context.Context,
	workspaceID string,
	channel string,
	threadID string,
) ([]store.NetworkThreadSessionTokenStats, error) {
	costStore, ok := h.NetworkStore.(networkThreadSessionCostStore)
	if !ok {
		return nil, nil
	}
	return costStore.ListNetworkThreadSessionTokenStats(
		ctx,
		store.NetworkThreadSessionTokenStatsQuery{
			WorkspaceID: workspaceID,
			Channel:     channel,
			ThreadID:    threadID,
		},
	)
}

func (h *BaseHandlers) networkThreadTaskLinks(
	ctx context.Context,
	workspaceID string,
	channel string,
	threadID string,
) ([]store.NetworkTaskThreadOrigin, error) {
	linkStore, ok := h.NetworkStore.(networkThreadTaskLinkStore)
	if !ok {
		return nil, nil
	}
	return linkStore.ListNetworkTaskThreadOrigins(
		ctx,
		store.NetworkTaskThreadOriginQuery{
			WorkspaceID: strings.TrimSpace(workspaceID),
			Channel:     strings.TrimSpace(channel),
			ThreadID:    strings.TrimSpace(threadID),
		},
	)
}
