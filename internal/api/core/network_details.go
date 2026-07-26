package core

import (
	"errors"
	"fmt"
	"net/http"

	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/store"

	"github.com/gin-gonic/gin"
)

type networkChannelAggregate struct {
	workspaceID                string
	channel                    string
	metadata                   *store.NetworkChannelEntry
	peerCount                  int
	localPeerCount             int
	sessionCount               int
	messageCount               int
	presenceCount              int
	lastActivityAt             *time.Time
	lastActivitySequence       int64
	lastPresenceAt             *time.Time
	lastPresenceSequence       int64
	lastMessageAt              *time.Time
	lastMessagePreview         string
	historicalParticipantCount int
	historicalParticipants     map[string]struct{}
}

type networkTimelineMessageView struct {
	entry store.NetworkMessageEntry
}

type networkMessageHistorySummary struct {
	conversation               []store.NetworkMessageEntry
	presenceMessages           []store.NetworkMessageEntry
	presenceCount              int
	lastPresenceAt             *time.Time
	historicalParticipantCount int
}

type networkChannelMetadataFields struct {
	createdAt         *time.Time
	purpose           string
	fanoutPolicy      string
	coordinatorPeerID string
	workspaceID       string
	createdBy         string
}

var errNetworkChannelNotFound = errors.New("api: network channel not found")

func (h *BaseHandlers) networkStoreRequired() (NetworkStore, error) {
	if h == nil || h.NetworkStore == nil {
		return nil, errors.New("api: network store is required")
	}
	return h.NetworkStore, nil
}

// CreateNetworkChannel validates and creates one new channel by starting a new session per selected agent.
func (h *BaseHandlers) CreateNetworkChannel(c *gin.Context) {
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

	var req contract.CreateNetworkChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode create network channel request: %w", h.transportName(), err),
		)
		return
	}
	scope, ok := h.resolveWorkspaceScope(c)
	if !ok {
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
	networkWorkspaceID := scope.NetworkWorkspaceID()
	req.WorkspaceID = networkWorkspaceID

	channel, purpose, agentNames, err := h.resolveCreateNetworkChannelRequest(
		c.Request.Context(),
		req,
		&scope.Resolved,
	)
	if err != nil {
		h.respondError(c, statusForCreateNetworkChannelError(err), err)
		return
	}
	fanoutPolicy, coordinatorPeerID, ok := h.createNetworkChannelFanoutFields(c, req)
	if !ok {
		return
	}

	entry := store.NetworkChannelEntry{
		Channel:           channel,
		WorkspaceID:       networkWorkspaceID,
		Purpose:           purpose,
		FanoutPolicy:      fanoutPolicy,
		CoordinatorPeerID: coordinatorPeerID,
		CreatedBy:         agentNames[0],
	}
	detail, status, err := h.provisionNetworkChannel(
		c.Request.Context(),
		service,
		networkStore,
		entry,
		agentNames,
	)
	if err != nil {
		h.respondError(c, status, err)
		return
	}

	c.JSON(http.StatusCreated, contract.CreateNetworkChannelResponse{Channel: detail})
}

func (h *BaseHandlers) createNetworkChannelFanoutFields(
	c *gin.Context,
	req contract.CreateNetworkChannelRequest,
) (string, string, bool) {
	fanoutPolicy := store.NormalizeNetworkFanoutPolicy(req.FanoutPolicy)
	coordinatorPeerID := strings.TrimSpace(req.CoordinatorPeerID)
	if err := store.ValidateNetworkChannelFanoutConfiguration(fanoutPolicy, coordinatorPeerID); err != nil {
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(err))
		return "", "", false
	}
	return fanoutPolicy, coordinatorPeerID, true
}

// NetworkChannel returns one network channel detail payload.
func (h *BaseHandlers) NetworkChannel(c *gin.Context) {
	service, err := h.networkServiceRequired()
	if err != nil {
		h.respondError(c, http.StatusServiceUnavailable, err)
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

	detail, err := h.networkChannelDetailPayload(c.Request.Context(), service, scope.NetworkWorkspaceID(), channel)
	if err != nil {
		if isNetworkChannelNotFound(err) {
			h.respondError(c, http.StatusNotFound, err)
			return
		}
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, contract.NetworkChannelResponse{Channel: detail})
}

// UpdateNetworkChannel mutates the channel metadata and delivery policy.
func (h *BaseHandlers) UpdateNetworkChannel(c *gin.Context) {
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
	var req contract.UpdateNetworkChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode update network channel request: %w", h.transportName(), err),
		)
		return
	}
	if req.Purpose == nil && req.FanoutPolicy == nil && req.CoordinatorPeerID == nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewNetworkValidationError(errors.New("network channel update must include at least one field")),
		)
		return
	}
	ref := scope.NetworkChannelRef(channel)
	patch := store.NetworkChannelPatch{UpdatedAt: h.nowUTC()}
	if req.Purpose != nil {
		purpose := strings.TrimSpace(*req.Purpose)
		patch.Purpose = &purpose
	}
	if req.FanoutPolicy != nil {
		fanoutPolicy := store.NormalizeNetworkFanoutPolicy(*req.FanoutPolicy)
		if err := store.ValidateNetworkFanoutPolicy(fanoutPolicy); err != nil {
			h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(err))
			return
		}
		patch.FanoutPolicy = &fanoutPolicy
	}
	if req.CoordinatorPeerID != nil {
		coordinatorPeerID := strings.TrimSpace(*req.CoordinatorPeerID)
		patch.CoordinatorPeerID = &coordinatorPeerID
	}
	if err := networkStore.PatchNetworkChannel(c.Request.Context(), ref, patch); err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	detail, err := h.networkChannelDetailPayload(c.Request.Context(), service, ref.WorkspaceID, ref.Channel)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, contract.NetworkChannelResponse{Channel: detail})
}
