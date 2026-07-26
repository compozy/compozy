package core

import (
	"fmt"

	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/gin-gonic/gin"
)

// ListBridgeSecretBindings returns the persisted secret bindings for one bridge instance.
func (h *BaseHandlers) ListBridgeSecretBindings(c *gin.Context) {
	bridges, ok := h.bridgeService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errBridgeServiceUnavailable)
		return
	}

	bindings, err := bridges.ListSecretBindings(c.Request.Context(), strings.TrimSpace(c.Param("id")))
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.BridgeSecretBindingsResponse{Bindings: bindings})
}

// PutBridgeSecretBinding creates or updates one bridge secret binding.
func (h *BaseHandlers) PutBridgeSecretBinding(c *gin.Context) {
	bridges, ok := h.bridgeService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errBridgeServiceUnavailable)
		return
	}

	var req contract.PutBridgeSecretBindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode bridge secret binding request: %w", h.transportName(), err),
		)
		return
	}

	binding, err := req.ToBridgeSecretBinding(c.Param("id"), c.Param("binding_name"))
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	if err := bridges.PutSecretBinding(c.Request.Context(), binding, req.SecretValue); err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.BridgeSecretBindingResponse{Binding: binding})
}

// DeleteBridgeSecretBinding removes one bridge secret binding row.
func (h *BaseHandlers) DeleteBridgeSecretBinding(c *gin.Context) {
	bridges, ok := h.bridgeService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errBridgeServiceUnavailable)
		return
	}

	if err := bridges.DeleteSecretBinding(c.Request.Context(), c.Param("id"), c.Param("binding_name")); err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	c.Status(http.StatusNoContent)
}

// TestBridgeDelivery resolves the typed outbound delivery target for one
// bridge instance without requiring a live platform adapter.
func (h *BaseHandlers) TestBridgeDelivery(c *gin.Context) {
	bridges, ok := h.bridgeService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errBridgeServiceUnavailable)
		return
	}

	var req contract.BridgeTestDeliveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode test delivery request: %w", h.transportName(), err),
		)
		return
	}

	targetReq, err := req.ToResolveDeliveryTargetRequest(c.Param("id"))
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}

	target, err := bridges.ResolveDeliveryTarget(c.Request.Context(), targetReq)
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.BridgeTestDeliveryResponse{
		Status:         "resolved",
		Message:        strings.TrimSpace(req.Message),
		DeliveryTarget: *target,
	})
}
