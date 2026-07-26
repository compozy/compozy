package core

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	presetspkg "github.com/compozy/agh/internal/notifications/presets"
	"github.com/gin-gonic/gin"
)

var errNotificationPresetServiceUnavailable = errors.New("notification preset service unavailable")

// ListNotificationPresets returns all persisted notification presets.
func (h *BaseHandlers) ListNotificationPresets(c *gin.Context) {
	service, ok := h.notificationPresetService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errNotificationPresetServiceUnavailable)
		return
	}
	query, err := parseNotificationPresetQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	presets, err := service.List(c.Request.Context(), query)
	if err != nil {
		h.respondError(c, StatusForNotificationPresetError(err), err)
		return
	}
	payloads := notificationPresetPayloads(presets)
	c.JSON(http.StatusOK, contract.NotificationPresetListResponse{
		Presets:     payloads,
		Total:       len(payloads),
		GeneratedAt: h.Now().UTC(),
	})
}

// GetNotificationPreset returns one notification preset.
func (h *BaseHandlers) GetNotificationPreset(c *gin.Context) {
	service, ok := h.notificationPresetService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errNotificationPresetServiceUnavailable)
		return
	}
	preset, err := service.Get(c.Request.Context(), c.Param("name"))
	if err != nil {
		h.respondError(c, StatusForNotificationPresetError(err), err)
		return
	}
	c.JSON(
		http.StatusOK,
		contract.NotificationPresetResponse{Preset: contract.NotificationPresetPayloadFromDomain(preset)},
	)
}

// CreateNotificationPreset creates one preset.
func (h *BaseHandlers) CreateNotificationPreset(c *gin.Context) {
	service, ok := h.notificationPresetService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errNotificationPresetServiceUnavailable)
		return
	}
	var req contract.CreateNotificationPresetRequest
	if err := decodeStrictBridgeJSON(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode create notification preset request: %w", h.transportName(), err),
		)
		return
	}
	preset, err := service.Create(c.Request.Context(), req.ToCreateRequest())
	if err != nil {
		h.respondError(c, StatusForNotificationPresetError(err), err)
		return
	}
	c.JSON(
		http.StatusCreated,
		contract.NotificationPresetResponse{Preset: contract.NotificationPresetPayloadFromDomain(preset)},
	)
}

// UpdateNotificationPreset mutates one preset.
func (h *BaseHandlers) UpdateNotificationPreset(c *gin.Context) {
	service, ok := h.notificationPresetService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errNotificationPresetServiceUnavailable)
		return
	}
	var req contract.UpdateNotificationPresetRequest
	if err := decodeStrictBridgeJSON(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode update notification preset request: %w", h.transportName(), err),
		)
		return
	}
	preset, err := service.Update(c.Request.Context(), c.Param("name"), req.ToUpdateRequest())
	if err != nil {
		h.respondError(c, StatusForNotificationPresetError(err), err)
		return
	}
	c.JSON(
		http.StatusOK,
		contract.NotificationPresetResponse{Preset: contract.NotificationPresetPayloadFromDomain(preset)},
	)
}

// DeleteNotificationPreset removes one preset.
func (h *BaseHandlers) DeleteNotificationPreset(c *gin.Context) {
	service, ok := h.notificationPresetService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errNotificationPresetServiceUnavailable)
		return
	}
	if err := service.Delete(c.Request.Context(), c.Param("name")); err != nil {
		h.respondError(c, StatusForNotificationPresetError(err), err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *BaseHandlers) notificationPresetService() (NotificationPresetService, bool) {
	if h == nil {
		return nil, false
	}
	if h.Notifications != nil {
		return h.Notifications, true
	}
	return nil, false
}

func parseNotificationPresetQuery(c *gin.Context) (presetspkg.Query, error) {
	query := presetspkg.Query{}
	if raw := strings.TrimSpace(c.Query("enabled")); raw != "" {
		enabled, parseErr := ParseOptionalBool(raw)
		if parseErr != nil {
			return presetspkg.Query{}, parseErr
		}
		query.Enabled = &enabled
	}
	if raw := strings.TrimSpace(c.Query("built_in")); raw != "" {
		builtIn, parseErr := ParseOptionalBool(raw)
		if parseErr != nil {
			return presetspkg.Query{}, parseErr
		}
		query.BuiltIn = &builtIn
	}
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		return presetspkg.Query{}, err
	}
	query.Name = c.Query("name")
	query.Limit = limit
	return query.Normalize(), nil
}

func notificationPresetPayloads(items []presetspkg.Preset) []contract.NotificationPresetPayload {
	payloads := make([]contract.NotificationPresetPayload, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, contract.NotificationPresetPayloadFromDomain(item))
	}
	return payloads
}
