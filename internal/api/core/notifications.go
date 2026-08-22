package core

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	presetspkg "github.com/compozy/compozy/internal/notifications/presets"
	profilepkg "github.com/compozy/compozy/internal/profile"
	taskpkg "github.com/compozy/compozy/internal/task"
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
	scope, profileName, err := h.notificationPresetProfileScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	enablement, ok := service.(NotificationPresetEnablementService)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errNotificationPresetServiceUnavailable)
		return
	}
	presets, err := enablement.ListForProfile(c.Request.Context(), query, scope.ProfileID)
	if err != nil {
		h.respondError(c, StatusForNotificationPresetError(err), err)
		return
	}
	payloads := notificationPresetPayloads(presets, profileName)
	c.JSON(http.StatusOK, contract.NotificationPresetListResponse{
		Presets:     payloads,
		Total:       len(payloads),
		GeneratedAt: h.Now().UTC(),
	})
}

// SetNotificationPresetEnablement changes one profile's preset exception state.
func (h *BaseHandlers) SetNotificationPresetEnablement(c *gin.Context) {
	service, ok := h.notificationPresetService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errNotificationPresetServiceUnavailable)
		return
	}
	enablement, ok := service.(NotificationPresetEnablementService)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errNotificationPresetServiceUnavailable)
		return
	}
	var req contract.SetNotificationPresetEnablementRequest
	if err := decodeStrictBridgeJSON(c, &req); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	profileName := strings.TrimSpace(req.Profile)
	if profileName == "" || h.Profiles == nil {
		respondProfileError(c, profilepkg.ErrInvalidInput)
		return
	}
	resolved, err := h.profileService().Resolve(c.Request.Context(), profilepkg.ResolveInput{
		Flag: profileName, Lens: profilepkg.Lens{Kind: profilepkg.SelectionLensGlobal},
	})
	if err != nil {
		respondProfileError(c, err)
		return
	}
	actor, err := h.notificationPresetMutationActor(c)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	preset, err := enablement.SetEnablement(
		c.Request.Context(), c.Param("name"), resolved.Profile.ID, resolved.Profile.Name,
		string(actor.Actor.Kind.Normalize()), actor.Actor.Ref, req.Enabled,
	)
	if err != nil {
		h.respondError(c, StatusForNotificationPresetError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.NotificationPresetEnablementPayload{
		Name: preset.Name, Profile: resolved.Profile.Name, Enabled: preset.Enabled,
	})
}

func (h *BaseHandlers) notificationPresetMutationActor(c *gin.Context) (taskpkg.ActorContext, error) {
	const action = "notifications.presets.enablement"
	if h.TaskActorContextResolver != nil {
		return h.TaskActorContextResolver(c, action)
	}
	return taskpkg.DeriveHumanActorContext(
		defaultTaskActorRef, taskOriginKindForTransport(h.transportName()), action,
	)
}

// GetNotificationPreset returns one notification preset.
func (h *BaseHandlers) GetNotificationPreset(c *gin.Context) {
	service, ok := h.notificationPresetService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errNotificationPresetServiceUnavailable)
		return
	}
	scope, profileName, err := h.notificationPresetProfileScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	enablement, ok := service.(NotificationPresetEnablementService)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errNotificationPresetServiceUnavailable)
		return
	}
	preset, err := enablement.GetForProfile(c.Request.Context(), c.Param("name"), scope.ProfileID)
	if err != nil {
		h.respondError(c, StatusForNotificationPresetError(err), err)
		return
	}
	c.JSON(
		http.StatusOK,
		contract.NotificationPresetResponse{
			Preset: contract.NotificationPresetPayloadFromDomain(preset, profileName),
		},
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
	scope, profileName, err := h.notificationPresetProfileScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	preset, err := service.Create(c.Request.Context(), req.ToCreateRequest())
	if err != nil {
		h.respondError(c, StatusForNotificationPresetError(err), err)
		return
	}
	if enablement, ok := service.(NotificationPresetEnablementService); ok {
		preset, err = enablement.GetForProfile(c.Request.Context(), preset.Name, scope.ProfileID)
		if err != nil {
			h.respondError(c, StatusForNotificationPresetError(err), err)
			return
		}
	}
	c.JSON(
		http.StatusCreated,
		contract.NotificationPresetResponse{
			Preset: contract.NotificationPresetPayloadFromDomain(preset, profileName),
		},
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
	scope, profileName, err := h.notificationPresetProfileScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	preset, err := service.Update(c.Request.Context(), c.Param("name"), req.ToUpdateRequest())
	if err != nil {
		h.respondError(c, StatusForNotificationPresetError(err), err)
		return
	}
	if enablement, ok := service.(NotificationPresetEnablementService); ok {
		preset, err = enablement.GetForProfile(c.Request.Context(), preset.Name, scope.ProfileID)
		if err != nil {
			h.respondError(c, StatusForNotificationPresetError(err), err)
			return
		}
	}
	c.JSON(
		http.StatusOK,
		contract.NotificationPresetResponse{
			Preset: contract.NotificationPresetPayloadFromDomain(preset, profileName),
		},
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

func notificationPresetPayloads(
	items []presetspkg.Preset,
	profileName string,
) []contract.NotificationPresetPayload {
	payloads := make([]contract.NotificationPresetPayload, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, contract.NotificationPresetPayloadFromDomain(item, profileName))
	}
	return payloads
}

func (h *BaseHandlers) notificationPresetProfileScope(
	c *gin.Context,
) (profilepkg.ReadScope, string, error) {
	scope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		return profilepkg.ReadScope{}, "", err
	}
	owners, err := h.profileOwnerIdentities(c.Request.Context())
	if err != nil {
		return profilepkg.ReadScope{}, "", err
	}
	owner, exists := owners[scope.ProfileID]
	if !exists || strings.TrimSpace(owner.Name) == "" {
		return profilepkg.ReadScope{}, "", profilepkg.ErrNotFound
	}
	return scope, owner.Name, nil
}
