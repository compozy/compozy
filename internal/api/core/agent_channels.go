package core

import (
	"context"

	"errors"
	"fmt"
	"net/http"

	"strings"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network"

	"github.com/compozy/agh/internal/store"
	"github.com/gin-gonic/gin"
)

const (
	agentActionContext      = "agent.context"
	agentActionChannelList  = "agent.ch.list"
	agentActionChannelRecv  = "agent.ch.recv"
	agentActionChannelSend  = "agent.ch.send"
	agentActionChannelReply = "agent.ch.reply"

	agentCoordinationExtKey = "coordination"
	agentChannelThreadID    = "thread_agent_channel"
)

// AgentContext returns the bounded situation payload for the validated caller session.
func (h *BaseHandlers) AgentContext(c *gin.Context) {
	caller, ok := h.requireAgentCaller(c, agentActionContext)
	if !ok {
		return
	}
	if h.AgentContextService == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: agent context service is not configured"))
		return
	}

	payload, err := h.AgentContextService.ContextForSession(c.Request.Context(), sessionInfoFromAgentCaller(caller))
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, contract.AgentContextResponse{
		Context: contract.NormalizeAgentContextPayload(&payload),
	})
}

// AgentChannels lists discoverable coordination channels for the validated caller.
func (h *BaseHandlers) AgentChannels(c *gin.Context) {
	caller, ok := h.requireAgentCaller(c, agentActionChannelList)
	if !ok {
		return
	}
	service, err := h.networkServiceRequired()
	if err != nil {
		h.respondError(c, http.StatusServiceUnavailable, err)
		return
	}

	channels, err := h.agentChannelPayloads(c.Request.Context(), caller, service)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.AgentChannelsResponse{Channels: channels})
}

// AgentChannelRecv returns queued channel messages for the validated caller, optionally waiting.
func (h *BaseHandlers) AgentChannelRecv(c *gin.Context) {
	caller, ok := h.requireAgentCaller(c, agentActionChannelRecv)
	if !ok {
		return
	}
	service, err := h.networkServiceRequired()
	if err != nil {
		h.respondError(c, http.StatusServiceUnavailable, err)
		return
	}
	channel := strings.TrimSpace(c.Param("channel"))
	if channel == "" {
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(errors.New("channel is required")))
		return
	}
	if err := network.ValidateChannel(channel); err != nil {
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(err))
		return
	}
	wait, err := parseBoolQuery(c, "wait")
	if err != nil {
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(err))
		return
	}
	limit, err := parsePositiveIntQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(err))
		return
	}

	envelopes, err := agentChannelInbox(
		c.Request.Context(),
		service,
		caller.Session.ID,
		channel,
		wait,
	)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	messages := agentChannelMessagesFromEnvelopes(envelopes, channel, limit)
	c.JSON(http.StatusOK, contract.AgentChannelMessagesResponse{Messages: messages})
}

// AgentChannelSend sends one coordination message using the validated caller identity.
func (h *BaseHandlers) AgentChannelSend(c *gin.Context) {
	caller, ok := h.requireAgentCaller(c, agentActionChannelSend)
	if !ok {
		return
	}
	service, err := h.networkServiceRequired()
	if err != nil {
		h.respondError(c, http.StatusServiceUnavailable, err)
		return
	}
	channel := strings.TrimSpace(c.Param("channel"))
	if channel == "" {
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(errors.New("channel is required")))
		return
	}

	var req contract.AgentChannelSendRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode agent channel send request: %w", h.transportName(), err),
		)
		return
	}
	if err := validateAgentChannelRequest(req.Body, req.Metadata, req); err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}

	ext, err := coordinationExt(req.Metadata)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	sendReq := network.SendRequest{
		SessionID: strings.TrimSpace(caller.Session.ID),
		Channel:   channel,
		Surface:   new(network.SurfaceThread),
		ThreadID:  ptrString(agentChannelThreadID),
		Kind:      network.KindSay,
		Body:      cloneRawMessage(req.Body),
		Ext:       ext,
	}
	if idempotencyKey := strings.TrimSpace(req.IdempotencyKey); idempotencyKey != "" {
		sendReq.ID = ptrString(idempotencyKey)
	}

	messageID, err := service.Send(c.Request.Context(), sendReq)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	c.JSON(http.StatusAccepted, contract.AgentChannelMessageResponse{
		Message: agentChannelMessageFromRequest(
			messageID,
			channel,
			caller.Session.ID,
			"",
			req.Body,
			req.Metadata,
			h.nowUTC(),
		),
	})
}

func (h *BaseHandlers) enrichAgentMePayload(
	ctx context.Context,
	caller agentidentity.Caller,
	payload *contract.AgentMePayload,
) {
	if payload == nil {
		return
	}
	if h != nil && h.AgentContextService != nil {
		contextPayload, err := h.AgentContextService.ContextForSession(ctx, sessionInfoFromAgentCaller(caller))
		if err == nil {
			payload.Workspace = contextPayload.Workspace
			payload.Capabilities = contextPayload.Capabilities.Capabilities
			payload.Limits = contextPayload.Limits
			if contextPayload.Task.Lease != nil {
				payload.ActiveTaskLeases = []contract.TaskRunLeaseSummaryPayload{*contextPayload.Task.Lease}
			}
			if contextPayload.CoordinationChannel.Channel != nil {
				payload.Channels = append(payload.Channels, *contextPayload.CoordinationChannel.Channel)
			}
		}
	}
	if h == nil {
		return
	}
	if coordinatorPayload, err := h.agentCoordinatorConfigPayload(ctx, caller.Session.WorkspaceID); err == nil {
		payload.Coordinator = coordinatorPayload
	}
	service, err := h.networkServiceRequired()
	if err != nil {
		if callerChannel := strings.TrimSpace(
			caller.Session.NetworkSpecSnapshot().ChannelID,
		); callerChannel != "" &&
			len(payload.Channels) == 0 {
			payload.Channels = []contract.CoordinationChannelPayload{
				contract.NormalizeCoordinationChannelPayload(coordinationChannelFromNetwork(
					callerChannel,
					caller.Session.WorkspaceID,
					store.NetworkChannelEntry{},
				)),
			}
		}
		return
	}
	channels, err := h.agentChannelPayloads(ctx, caller, service)
	if err == nil {
		payload.Channels = mergeCoordinationChannels(payload.Channels, channels)
	}
}
