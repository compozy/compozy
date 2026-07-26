package core

import (
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/store"
	"github.com/gin-gonic/gin"
)

// NetworkThreads returns one stable cursor page of public-thread summaries.
func (h *BaseHandlers) NetworkThreads(c *gin.Context) {
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
	query, err := parseNetworkThreadQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	page, err := h.NetworkStore.ListThreads(
		c.Request.Context(),
		scope.NetworkChannelRef(channel),
		query,
	)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.NetworkThreadsResponse{
		Threads: NetworkThreadSummaryPayloadsFromStore(page.Threads),
		Page:    networkThreadPagePayload(page),
	})
}

// NetworkThreadMessages returns one stable cursor page for a public thread.
func (h *BaseHandlers) NetworkThreadMessages(c *gin.Context) {
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
	h.respondNetworkConversationMessages(c, store.NetworkConversationRef{
		WorkspaceID: scope.NetworkWorkspaceID(),
		Channel:     channel,
		Surface:     store.NetworkSurfaceThread,
		ThreadID:    strings.TrimSpace(c.Param("thread_id")),
	})
}

// NetworkDirectRooms returns one stable cursor page of direct-room summaries.
func (h *BaseHandlers) NetworkDirectRooms(c *gin.Context) {
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
	query, err := parseNetworkDirectRoomQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	page, err := h.NetworkStore.ListDirectRooms(
		c.Request.Context(),
		scope.NetworkChannelRef(channel),
		query,
	)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.NetworkDirectRoomsResponse{
		Directs: NetworkDirectRoomPayloadsFromStore(page.Directs),
		Page:    networkDirectRoomPagePayload(page),
	})
}

// NetworkDirectRoomMessages returns one stable cursor page for a direct room.
func (h *BaseHandlers) NetworkDirectRoomMessages(c *gin.Context) {
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
	h.respondNetworkConversationMessages(c, store.NetworkConversationRef{
		WorkspaceID: scope.NetworkWorkspaceID(),
		Channel:     channel,
		Surface:     store.NetworkSurfaceDirect,
		DirectID:    strings.TrimSpace(c.Param("direct_id")),
	})
}

func (h *BaseHandlers) respondNetworkConversationMessages(c *gin.Context, ref store.NetworkConversationRef) {
	if err := ref.Validate(); err != nil {
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(err))
		return
	}
	query, err := parseNetworkConversationMessageQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	messages, err := h.NetworkStore.ListConversationMessages(
		c.Request.Context(),
		ref,
		overfetchNetworkConversationMessageQuery(query),
	)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	payload, page := trimNetworkMessagePayloadPage(
		NetworkConversationMessagePayloadsFromStore(messages),
		query.AfterMessageID,
		query.Limit,
	)
	switch ref.Surface {
	case store.NetworkSurfaceThread:
		c.JSON(http.StatusOK, contract.NetworkThreadMessagesResponse{Messages: payload, Page: page})
	case store.NetworkSurfaceDirect:
		c.JSON(http.StatusOK, contract.NetworkDirectRoomMessagesResponse{Messages: payload, Page: page})
	default:
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(errors.New("surface is required")))
	}
}

func parseNetworkThreadQuery(c *gin.Context) (store.NetworkThreadQuery, error) {
	limit, err := parsePositiveIntQuery(c)
	if err != nil {
		return store.NetworkThreadQuery{}, NewNetworkValidationError(err)
	}
	hasWork, err := parseNetworkHasWorkQuery(c)
	if err != nil {
		return store.NetworkThreadQuery{}, err
	}
	query := store.NetworkThreadQuery{
		Search:    strings.TrimSpace(c.Query("query")),
		SessionID: strings.TrimSpace(c.Query("session_id")),
		Sort:      strings.TrimSpace(c.Query("sort")),
		HasWork:   hasWork,
		Limit:     limit,
		After:     strings.TrimSpace(c.Query("after")),
	}
	if err := query.Validate(); err != nil {
		return store.NetworkThreadQuery{}, NewNetworkValidationError(err)
	}
	return query, nil
}

func parseNetworkDirectRoomQuery(c *gin.Context) (store.NetworkDirectRoomQuery, error) {
	limit, err := parsePositiveIntQuery(c)
	if err != nil {
		return store.NetworkDirectRoomQuery{}, NewNetworkValidationError(err)
	}
	hasWork, err := parseNetworkHasWorkQuery(c)
	if err != nil {
		return store.NetworkDirectRoomQuery{}, err
	}
	query := store.NetworkDirectRoomQuery{
		Search:    strings.TrimSpace(c.Query("query")),
		SessionID: strings.TrimSpace(c.Query("session_id")),
		Sort:      strings.TrimSpace(c.Query("sort")),
		HasWork:   hasWork,
		Limit:     limit,
		After:     strings.TrimSpace(c.Query("after")),
	}
	if err := query.Validate(); err != nil {
		return store.NetworkDirectRoomQuery{}, NewNetworkValidationError(err)
	}
	return query, nil
}

func parseNetworkHasWorkQuery(c *gin.Context) (*bool, error) {
	raw, exists := c.GetQuery("has_work")
	if !exists {
		return nil, nil
	}
	value, err := ParseOptionalBool(raw)
	if err != nil {
		return nil, NewNetworkValidationError(err)
	}
	return &value, nil
}

func parseNetworkConversationMessageQuery(c *gin.Context) (store.NetworkConversationMessageQuery, error) {
	limit, err := parsePositiveIntQuery(c)
	if err != nil {
		return store.NetworkConversationMessageQuery{}, NewNetworkValidationError(err)
	}
	query := store.NetworkConversationMessageQuery{
		BeforeMessageID: strings.TrimSpace(c.Query("before")),
		AfterMessageID:  strings.TrimSpace(c.Query("after")),
		Kind:            strings.TrimSpace(c.Query("kind")),
		WorkID:          strings.TrimSpace(c.Query("work_id")),
		Limit:           store.NormalizeNetworkMessageLimit(limit),
	}
	if err := query.Validate(); err != nil {
		return store.NetworkConversationMessageQuery{}, NewNetworkValidationError(err)
	}
	return query, nil
}
