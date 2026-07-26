package core

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/store"

	"github.com/gin-gonic/gin"
)

// NetworkWork returns one network work row by work_id.
func (h *BaseHandlers) NetworkWork(c *gin.Context) {
	if _, ok := h.requireNetworkReadDependencies(c); !ok {
		return
	}
	workID := strings.TrimSpace(c.Param("work_id"))
	if err := network.ValidateWorkID(workID); err != nil {
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(err))
		return
	}
	scope, ok := h.resolveWorkspaceScope(c)
	if !ok {
		return
	}
	work, err := h.NetworkStore.GetWork(c.Request.Context(), scope.NetworkWorkspaceID(), workID)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.NetworkWorkResponse{Work: NetworkWorkPayloadFromStore(work)})
}

func (h *BaseHandlers) requireNetworkReadDependencies(c *gin.Context) (NetworkService, bool) {
	service, err := h.networkServiceRequired()
	if err != nil {
		h.respondError(c, http.StatusServiceUnavailable, err)
		return nil, false
	}
	if _, err := h.networkStoreRequired(); err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return nil, false
	}
	return service, true
}

func networkThreadPromotionDigest(
	messages []store.NetworkConversationMessage,
	originMessageID string,
) (string, []string, bool) {
	target := strings.TrimSpace(originMessageID)
	sourceIDs := make([]string, 0, len(messages))
	var builder strings.Builder
	found := false
	for _, message := range messages {
		messageID := strings.TrimSpace(message.MessageID)
		if messageID == "" {
			continue
		}
		sourceIDs = append(sourceIDs, messageID)
		if messageID == target {
			found = true
		}
		if builder.Len() >= 4000 {
			continue
		}
		line := networkThreadPromotionMessageLine(message)
		if line == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(line)
	}
	digest := strings.TrimSpace(builder.String())
	if len(digest) > 4000 {
		digest = truncateNetworkPromotionDigest(digest, 4000)
	}
	return digest, sourceIDs, found
}

func truncateNetworkPromotionDigest(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return strings.TrimSpace(value)
	}
	truncated := value[:maxBytes]
	for !utf8.ValidString(truncated) && truncated != "" {
		truncated = truncated[:len(truncated)-1]
	}
	return strings.TrimSpace(truncated)
}

func networkThreadPromotionMessageLine(message store.NetworkConversationMessage) string {
	preview := strings.TrimSpace(message.PreviewText)
	if preview == "" {
		preview = strings.TrimSpace(message.Text)
	}
	if preview == "" {
		return ""
	}
	peer := firstNonEmpty(strings.TrimSpace(message.PeerFrom), strings.TrimSpace(message.SessionID), unknownValue)
	return peer + ": " + preview
}
