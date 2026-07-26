package core

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	bundlepkg "github.com/compozy/agh/internal/bundles"
	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/resources"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) ListBundleCatalog(c *gin.Context) {
	if !bundleServiceRequired(c, h) {
		return
	}

	items, err := h.Bundles.Catalog(c.Request.Context())
	if err != nil {
		h.respondError(c, StatusForBundleError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.BundlesCatalogResponse{Bundles: BundleCatalogPayloads(items)})
}

func (h *BaseHandlers) PreviewBundleActivation(c *gin.Context) {
	req, ok := h.bindBundleActivateRequest(c)
	if !ok {
		return
	}

	preview, err := h.Bundles.PreviewActivation(c.Request.Context(), req)
	if err != nil {
		h.respondError(c, StatusForBundleError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.BundlePreviewResponse{Activation: BundleActivationPayload(preview)})
}

func (h *BaseHandlers) ListBundleActivations(c *gin.Context) {
	if !bundleServiceRequired(c, h) {
		return
	}

	items, err := h.Bundles.ListActivations(c.Request.Context())
	if err != nil {
		h.respondError(c, StatusForBundleError(err), err)
		return
	}
	payload := make([]contract.BundleActivationPayload, 0, len(items))
	for _, item := range items {
		payload = append(payload, BundleActivationPayload(item))
	}
	c.JSON(http.StatusOK, contract.BundleActivationsResponse{Activations: payload})
}

func (h *BaseHandlers) ActivateBundle(c *gin.Context) {
	req, ok := h.bindBundleActivateRequest(c)
	if !ok {
		return
	}

	item, err := h.Bundles.Activate(c.Request.Context(), req)
	if err != nil {
		h.respondError(c, StatusForBundleError(err), err)
		return
	}
	c.JSON(http.StatusCreated, contract.BundleActivationResponse{Activation: BundleActivationPayload(item)})
}

func (h *BaseHandlers) GetBundleActivation(c *gin.Context) {
	if !bundleServiceRequired(c, h) {
		return
	}

	item, err := h.Bundles.GetActivation(c.Request.Context(), strings.TrimSpace(c.Param("id")))
	if err != nil {
		h.respondError(c, StatusForBundleError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.BundleActivationResponse{Activation: BundleActivationPayload(item)})
}

func (h *BaseHandlers) UpdateBundleActivation(c *gin.Context) {
	if !bundleServiceRequired(c, h) {
		return
	}

	var req contract.UpdateBundleActivationRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	if req.ExpectedVersion <= 0 {
		h.respondError(c, http.StatusBadRequest, errors.New("bundles: expected_version must be positive"))
		return
	}

	item, err := h.Bundles.UpdateActivation(c.Request.Context(), bundlepkg.UpdateActivationRequest{
		ID:                        strings.TrimSpace(c.Param("id")),
		ExpectedVersion:           req.ExpectedVersion,
		ConfirmNetworkRequirement: req.ConfirmNetworkRequirement,
	})
	if err != nil {
		h.respondError(c, StatusForBundleError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.BundleActivationResponse{Activation: BundleActivationPayload(item)})
}

func (h *BaseHandlers) DeleteBundleActivation(c *gin.Context) {
	if !bundleServiceRequired(c, h) {
		return
	}

	if err := h.Bundles.Deactivate(c.Request.Context(), strings.TrimSpace(c.Param("id"))); err != nil {
		h.respondError(c, StatusForBundleError(err), err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *BaseHandlers) BundleNetworkSettings(c *gin.Context) {
	if !bundleServiceRequired(c, h) {
		return
	}

	settings, err := h.Bundles.NetworkSettings(c.Request.Context())
	if err != nil {
		h.respondError(c, StatusForBundleError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.BundleNetworkSettingsResponse{
		Network: contract.BundleNetworkSettingsPayload{
			DeclaredChannels: DeclaredNetworkChannelPayloads(settings.DeclaredChannels),
		},
	})
}

func (h *BaseHandlers) bindBundleActivateRequest(c *gin.Context) (bundlepkg.ActivateRequest, bool) {
	if !bundleServiceRequired(c, h) {
		return bundlepkg.ActivateRequest{}, false
	}

	var req contract.ActivateBundleRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return bundlepkg.ActivateRequest{}, false
	}

	return bundlepkg.ActivateRequest{
		ExtensionName:             strings.TrimSpace(req.ExtensionName),
		BundleName:                strings.TrimSpace(req.BundleName),
		ProfileName:               strings.TrimSpace(req.ProfileName),
		Scope:                     bundlepkg.Scope(strings.TrimSpace(req.Scope)).Normalize(),
		Workspace:                 strings.TrimSpace(req.Workspace),
		ConfirmNetworkRequirement: req.ConfirmNetworkRequirement,
	}, true
}

func bundleServiceRequired(c *gin.Context, h *BaseHandlers) bool {
	err := errors.New("api: bundle service is not configured")
	if h == nil {
		RespondError(c, http.StatusServiceUnavailable, err, false)
		return false
	}
	if h.Bundles == nil {
		h.respondError(c, http.StatusServiceUnavailable, err)
		return false
	}
	return true
}

func BundleCatalogPayloads(items []bundlepkg.CatalogEntry) []contract.BundleCatalogPayload {
	payload := make([]contract.BundleCatalogPayload, 0, len(items))
	for _, item := range items {
		profiles := make([]contract.BundleProfileCatalogPayload, 0, len(item.Bundle.Profiles))
		for _, profile := range item.Bundle.Profiles {
			profiles = append(profiles, contract.BundleProfileCatalogPayload{
				Name:           strings.TrimSpace(profile.Name),
				Description:    strings.TrimSpace(profile.Description),
				PrimaryChannel: strings.TrimSpace(profile.Channels.Primary),
				Channels:       bundleChannelPayloads(profile.Channels),
				LayoutCount:    len(profile.Layouts),
				AgentCount:     len(profile.Agents),
				JobCount:       len(profile.Jobs),
				TriggerCount:   len(profile.Triggers),
				BridgeCount:    len(profile.Bridges),
			})
		}
		payload = append(payload, contract.BundleCatalogPayload{
			ExtensionName: strings.TrimSpace(item.ExtensionName),
			BundleName:    strings.TrimSpace(item.Bundle.Name),
			Description:   strings.TrimSpace(item.Bundle.Description),
			Profiles:      profiles,
		})
	}
	return payload
}

func BundleActivationPayload(item bundlepkg.ActivationPreview) contract.BundleActivationPayload {
	agents := make([]contract.BundleAgentPayload, 0, len(item.Profile.Agents))
	jobs := make([]contract.BundleJobPayload, 0, len(item.Profile.Jobs))
	triggers := make([]contract.BundleTriggerPayload, 0, len(item.Profile.Triggers))
	bridges := make([]contract.BundleBridgePayload, 0, len(item.Profile.Bridges))
	for _, agent := range item.Profile.Agents {
		agents = append(agents, contract.BundleAgentPayload{
			ID:           bundlepkgStableID("agt", item.Activation.ID, agent.Agent.Name),
			Name:         strings.TrimSpace(agent.Agent.Name),
			Provider:     strings.TrimSpace(agent.Agent.Provider),
			Model:        strings.TrimSpace(agent.Agent.Model),
			CategoryPath: append([]string(nil), agent.Agent.CategoryPath...),
			HasSoul:      agent.Soul != nil,
			HasHeartbeat: agent.Heartbeat != nil,
		})
	}
	for _, job := range item.Profile.Jobs {
		jobs = append(jobs, contract.BundleJobPayload{
			ID:        bundlepkgStableID("job", item.Activation.ID, job.Name),
			Name:      strings.TrimSpace(job.Name),
			AgentName: strings.TrimSpace(job.AgentName),
			Enabled:   job.Enabled,
		})
	}
	for _, trigger := range item.Profile.Triggers {
		triggers = append(triggers, contract.BundleTriggerPayload{
			ID:        bundlepkgStableID("trg", item.Activation.ID, trigger.Name),
			Name:      strings.TrimSpace(trigger.Name),
			AgentName: strings.TrimSpace(trigger.AgentName),
			Event:     strings.TrimSpace(trigger.Event),
			Enabled:   trigger.Enabled,
		})
	}
	for _, bridge := range item.Profile.Bridges {
		slots := make([]contract.BundleBridgeSecretSlotPayload, 0, len(bridge.SecretSlots))
		for _, slot := range bridge.SecretSlots {
			slots = append(slots, contract.BundleBridgeSecretSlotPayload{
				Name:        strings.TrimSpace(slot.Name),
				Kind:        strings.TrimSpace(slot.Kind),
				Description: strings.TrimSpace(slot.Description),
			})
		}
		bridges = append(bridges, contract.BundleBridgePayload{
			ID:            bundlepkgStableID("bri", item.Activation.ID, bridge.Name),
			Name:          strings.TrimSpace(bridge.Name),
			ExtensionName: strings.TrimSpace(bridge.ExtensionName),
			Platform:      strings.TrimSpace(bridge.Platform),
			DisplayName:   strings.TrimSpace(bridge.DisplayName),
			SecretSlots:   slots,
		})
	}
	return contract.BundleActivationPayload{
		ID:                            strings.TrimSpace(item.Activation.ID),
		Version:                       item.Activation.Version,
		ExtensionName:                 strings.TrimSpace(item.Activation.ExtensionName),
		BundleName:                    strings.TrimSpace(item.Bundle.Name),
		BundleDescription:             strings.TrimSpace(item.Bundle.Description),
		ProfileName:                   strings.TrimSpace(item.Profile.Name),
		ProfileDescription:            strings.TrimSpace(item.Profile.Description),
		Scope:                         string(item.Activation.Scope),
		WorkspaceID:                   strings.TrimSpace(item.Activation.WorkspaceID),
		NetworkRequirementDigest:      strings.TrimSpace(item.Activation.NetworkRequirementDigest),
		NetworkRequirementConfirmedBy: strings.TrimSpace(item.Activation.ConfirmedBy),
		NetworkRequirementConfirmedAt: strings.TrimSpace(item.Activation.ConfirmedAt),
		Channels:                      bundleChannelPayloads(item.Profile.Channels),
		Layouts:                       bundleLayoutPayloads(item.Activation.ID, item.Profile.Layouts),
		Agents:                        agents,
		Jobs:                          jobs,
		Triggers:                      triggers,
		Bridges:                       bridges,
		Inventory:                     bundleInventoryPayloads(item.Inventory),
		SpecDrift:                     item.SpecDrift,
		CreatedAt:                     item.Activation.CreatedAt,
		UpdatedAt:                     item.Activation.UpdatedAt,
	}
}

func bundleLayoutPayloads(
	activationID string,
	items []extensionpkg.BundleLayout,
) []contract.BundleLayoutPayload {
	payload := make([]contract.BundleLayoutPayload, 0, len(items))
	for _, item := range items {
		slots := make([]string, 0, len(item.Layout.ParticipantSlots))
		for _, slot := range item.Layout.ParticipantSlots {
			slots = append(slots, strings.TrimSpace(string(slot)))
		}
		payload = append(payload, contract.BundleLayoutPayload{
			ID:               bundlepkgStableID("lay", activationID, item.Layout.ID),
			DisplayName:      strings.TrimSpace(item.Layout.DisplayName),
			AspectVariant:    strings.TrimSpace(string(item.Layout.AspectVariant)),
			ParticipantSlots: slots,
			OverflowPolicy:   strings.TrimSpace(string(item.Layout.OverflowPolicy)),
		})
	}
	return payload
}

func bundleInventoryPayloads(items []bundlepkg.InventoryItem) []contract.BundleInventoryPayload {
	payload := make([]contract.BundleInventoryPayload, 0, len(items))
	for _, item := range items {
		payload = append(payload, contract.BundleInventoryPayload{
			ResourceKind: strings.TrimSpace(item.ResourceKind),
			ResourceID:   strings.TrimSpace(item.ResourceID),
			ResourceName: strings.TrimSpace(item.ResourceName),
		})
	}
	return payload
}

func DeclaredNetworkChannelPayloads(items []bundlepkg.DeclaredChannel) []contract.DeclaredNetworkChannelPayload {
	payload := make([]contract.DeclaredNetworkChannelPayload, 0, len(items))
	for _, item := range items {
		payload = append(payload, contract.DeclaredNetworkChannelPayload{
			ActivationID:  strings.TrimSpace(item.ActivationID),
			ExtensionName: strings.TrimSpace(item.ExtensionName),
			BundleName:    strings.TrimSpace(item.BundleName),
			ProfileName:   strings.TrimSpace(item.ProfileName),
			WorkspaceID:   strings.TrimSpace(item.WorkspaceID),
			Name:          strings.TrimSpace(item.Name),
			Description:   strings.TrimSpace(item.Description),
			Primary:       item.Primary,
		})
	}
	return payload
}

func bundleChannelPayloads(cfg extensionpkg.BundleChannelsConfig) []contract.BundleChannelPayload {
	payload := make([]contract.BundleChannelPayload, 0, len(cfg.Items))
	primary := strings.TrimSpace(cfg.Primary)
	for _, item := range cfg.Items {
		payload = append(payload, contract.BundleChannelPayload{
			Name:        strings.TrimSpace(item.Name),
			Description: strings.TrimSpace(item.Description),
			Primary:     strings.TrimSpace(item.Name) == primary,
		})
	}
	return payload
}

func StatusForBundleError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, bundlepkg.ErrActivationNotFound),
		errors.Is(err, bundlepkg.ErrBundleNotFound),
		errors.Is(err, bundlepkg.ErrProfileNotFound),
		errors.Is(err, extensionpkg.ErrExtensionNotFound):
		return http.StatusNotFound
	case errors.Is(err, bundlepkg.ErrAgentConflict),
		errors.Is(err, extensionpkg.ErrExtensionHasActiveBundles),
		errors.Is(err, bundlepkg.ErrNetworkRequirementConfirmationRequired),
		errors.Is(err, resources.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, bundlepkg.ErrAgentReferenceNotFound),
		errors.Is(err, aghconfig.ErrAgentNameReserved):
		return http.StatusUnprocessableEntity
	case errors.Is(err, bundlepkg.ErrWebhookUnsupported),
		errors.Is(err, resources.ErrValidation),
		errors.Is(err, extensionpkg.ErrBundleInvalid):
		return http.StatusBadRequest
	case errors.Is(err, workspacepkg.ErrWorkspaceNotFound),
		errors.Is(err, workspacepkg.ErrWorkspaceRootMissing):
		return StatusForWorkspaceError(err)
	default:
		return http.StatusInternalServerError
	}
}

func bundlepkgStableID(prefix string, parts ...string) string {
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized = append(normalized, strings.TrimSpace(part))
	}
	sum := sha256.Sum256([]byte(strings.Join(normalized, "\n")))
	return prefix + "_" + hex.EncodeToString(sum[:8])
}
