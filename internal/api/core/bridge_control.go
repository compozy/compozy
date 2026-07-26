package core

import (
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	redactpkg "github.com/compozy/agh/internal/redact"
	storepkg "github.com/compozy/agh/internal/store"
	"github.com/gin-gonic/gin"
)

// VerifyBridge runs live provider checks without changing bridge lifecycle state.
func (h *BaseHandlers) VerifyBridge(c *gin.Context) {
	bridges, instance, ok := h.bridgeControlInstance(c)
	if !ok {
		return
	}
	response, err := bridges.CheckBridge(c.Request.Context(), instance.ExtensionName, bridgepkg.BridgeCheckRequest{
		BridgeInstanceID: instance.ID,
	})
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.BridgeVerifyResponse{
		BridgeInstanceID: instance.ID,
		Checks:           response.Checks,
	})
}

// RegisterBridgeWebhook asks the owning adapter to register its configured callback.
func (h *BaseHandlers) RegisterBridgeWebhook(c *gin.Context) {
	bridges, instance, ok := h.bridgeControlInstance(c)
	if !ok {
		return
	}
	response, err := bridges.RegisterBridgeWebhook(
		c.Request.Context(),
		instance.ExtensionName,
		bridgepkg.BridgeWebhookRegistrationRequest{BridgeInstanceID: instance.ID},
	)
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.BridgeWebhookRegistrationResponse{
		BridgeInstanceID: instance.ID,
		Status:           response.Status,
		Remediation:      response.Remediation,
	})
}

// SendBridgeTest performs one synchronous delivery through the real provider adapter.
func (h *BaseHandlers) SendBridgeTest(c *gin.Context) {
	bridges, ok := h.bridgeService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errBridgeServiceUnavailable)
		return
	}
	var request contract.BridgeSendTestRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	message := request.NormalizedMessage()
	if message == "" {
		h.respondError(c, http.StatusBadRequest, bridgeControlRequestError("send-test message is required"))
		return
	}
	targetRequest, err := request.ToResolveDeliveryTargetRequest(c.Param("id"))
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	target, err := bridges.ResolveDeliveryTarget(c.Request.Context(), targetRequest)
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	instance, err := bridges.GetInstance(c.Request.Context(), strings.TrimSpace(c.Param("id")))
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	routingKey, err := bridgepkg.BuildRoutingKey(*instance, bridgepkg.RoutingDimensions{
		PeerID:   target.PeerID,
		ThreadID: target.ThreadID,
		GroupID:  target.GroupID,
	})
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	deliveryID := storepkg.NewID("del")
	deliveryRequest := bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
		DeliveryID:       deliveryID,
		BridgeInstanceID: instance.ID,
		RoutingKey:       routingKey,
		DeliveryTarget:   *target,
		Seq:              1,
		EventType:        bridgepkg.DeliveryEventTypeFinal,
		Content:          bridgepkg.MessageContent{Text: message},
		Final:            true,
		Operation:        bridgepkg.DeliveryOperationPost,
	}}
	if err := deliveryRequest.Validate(); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	ack, err := bridges.DeliverBridge(c.Request.Context(), instance.ExtensionName, deliveryRequest)
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	if err := ack.ValidateFor(deliveryRequest.Event); err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	status, detail := bridgeSendTestOutcome(ack)
	c.JSON(http.StatusOK, contract.BridgeSendTestResponse{
		Status:           status,
		BridgeInstanceID: instance.ID,
		DeliveryID:       deliveryID,
		RemoteMessageID:  ack.RemoteMessageID,
		DeliveryTarget:   *target,
		Error:            publicDeliveryAckError(detail),
	})
}

func bridgeSendTestOutcome(
	ack bridgepkg.DeliveryAck,
) (contract.BridgeSendTestStatus, *bridgepkg.DeliveryErrorDetail) {
	if ack.Outcome.Normalize() == bridgepkg.DeliveryAckOutcomeCommittedResultUnavailable {
		return contract.BridgeSendTestStatusCommittedResultUnavailable, ack.Error
	}
	if strings.TrimSpace(ack.RemoteMessageID) == "" {
		return contract.BridgeSendTestStatusCommittedResultUnavailable, &bridgepkg.DeliveryErrorDetail{
			Message: bridgepkg.DeliveryResultUnavailableMessage,
		}
	}
	return contract.BridgeSendTestStatusDelivered, nil
}

func publicDeliveryAckError(detail *bridgepkg.DeliveryErrorDetail) *bridgepkg.DeliveryErrorDetail {
	if detail == nil {
		return nil
	}
	return &bridgepkg.DeliveryErrorDetail{Message: redactpkg.String(strings.TrimSpace(detail.Message))}
}

func (h *BaseHandlers) bridgeControlInstance(
	c *gin.Context,
) (BridgeService, *bridgepkg.BridgeInstance, bool) {
	bridges, ok := h.bridgeService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errBridgeServiceUnavailable)
		return nil, nil, false
	}
	instance, err := bridges.GetInstance(c.Request.Context(), strings.TrimSpace(c.Param("id")))
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return nil, nil, false
	}
	return bridges, instance, true
}

type bridgeControlRequestError string

func (e bridgeControlRequestError) Error() string { return string(e) }
