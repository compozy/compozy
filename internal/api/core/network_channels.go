package core

import (
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

// NetworkChannels returns the active runtime channels and, when requested, a
// bounded cross-channel recent-conversation projection in the same request.
func (h *BaseHandlers) NetworkChannels(c *gin.Context) {
	service, err := h.networkServiceRequired()
	if err != nil {
		h.respondError(c, http.StatusServiceUnavailable, err)
		return
	}
	scope, ok := h.resolveWorkspaceScope(c)
	if !ok {
		return
	}
	readScope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	recentLimit, err := ParseOptionalInt(c.Query("recent_limit"))
	if err != nil || recentLimit < 0 {
		if err == nil {
			err = errors.New("recent_limit must be zero or positive")
		}
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(err))
		return
	}

	workspaceID := scope.NetworkWorkspaceID()
	channels, err := h.networkChannelPayloads(c.Request.Context(), service, readScope, workspaceID)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	recents, err := h.networkRecentPayloads(c, readScope, workspaceID, recentLimit)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.NetworkChannelsResponse{Channels: channels, Recents: recents})
}

func (h *BaseHandlers) networkRecentPayloads(
	c *gin.Context,
	readScope store.ReadScope,
	workspaceID string,
	limit int,
) ([]contract.NetworkRecentPayload, error) {
	if limit <= 0 {
		return nil, nil
	}
	recentStore, ok := h.NetworkStore.(store.NetworkRecentStore)
	if !ok {
		return nil, errors.New("api: network recent store is required")
	}
	recents, err := recentStore.ListNetworkRecents(c.Request.Context(), store.NetworkRecentQuery{
		ReadScope:   readScope,
		WorkspaceID: strings.TrimSpace(workspaceID),
		Limit:       limit,
	})
	if err != nil {
		return nil, err
	}
	payloads := make([]contract.NetworkRecentPayload, 0, len(recents))
	for _, recent := range recents {
		activity := recent.LastActivityAt.UTC()
		payloads = append(payloads, contract.NetworkRecentPayload{
			ProfileID:          strings.TrimSpace(recent.ProfileID),
			ProfileName:        strings.TrimSpace(recent.ProfileName),
			ProfileColor:       strings.TrimSpace(recent.ProfileColor),
			ProfileIcon:        strings.TrimSpace(recent.ProfileIcon),
			ProfileEmoji:       strings.TrimSpace(recent.ProfileEmoji),
			ProfileArchived:    recent.ProfileArchived,
			Channel:            strings.TrimSpace(recent.Channel),
			Surface:            strings.TrimSpace(recent.Surface),
			ContainerID:        strings.TrimSpace(recent.ContainerID),
			LastActivityAt:     &activity,
			LastMessagePreview: strings.TrimSpace(recent.LastMessagePreview),
			Title:              strings.TrimSpace(recent.Title),
			ParticipantCount:   recent.ParticipantCount,
			SessionA:           strings.TrimSpace(recent.SessionA),
			SessionB:           strings.TrimSpace(recent.SessionB),
		})
	}
	return payloads, nil
}
