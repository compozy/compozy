package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	automationpkg "github.com/compozy/compozy/internal/automation"
	"github.com/gin-gonic/gin"
)

// ListAutomationTriggers returns the filtered automation trigger list.
func (h *BaseHandlers) ListAutomationTriggers(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}
	projectionCtx, ok := h.requireGatewayIngressReadContext(c, "automation.triggers.list")
	if !ok {
		return
	}

	readScope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	query, err := ParseAutomationTriggerListQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, NewAutomationValidationError(err))
		return
	}
	query.ReadScope = readScope

	page, err := manager.ListTriggers(c.Request.Context(), query)
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	payloads, err := h.triggerPayloads(projectionCtx, page.Triggers)
	if err != nil {
		h.respondGatewayError(c, err)
		return
	}
	if err := h.decorateAutomationTriggerOwners(c.Request.Context(), payloads); err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.TriggersResponse{
		Triggers: payloads,
		Page: contract.CountedCursorPagePayload{
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
			Total:      page.Total,
			Limit:      page.Limit,
		},
	})
}

// CreateAutomationTrigger stores a new dynamic automation trigger definition.
func (h *BaseHandlers) CreateAutomationTrigger(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}
	mutationScope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}

	var req contract.CreateTriggerRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewAutomationValidationError(
				fmt.Errorf("%s: decode automation trigger create request: %w", h.transportName(), err),
			),
		)
		return
	}

	trigger := triggerFromCreateRequest(req)
	trigger.ProfileID = mutationScope.ProfileID
	webhookSecret := webhookSecretWriteFromCreateRequest(req)
	validationTrigger := trigger
	if strings.TrimSpace(validationTrigger.WebhookSecretRef) == "" && webhookSecret.Value != nil {
		validationTrigger.WebhookSecretRef = "vault:automation/triggers/create-validation/webhook-secret"
	}
	if err := validationTrigger.Validate("trigger"); err != nil {
		h.respondError(c, http.StatusBadRequest, NewAutomationValidationError(err))
		return
	}
	if err := automationpkg.ValidateTriggerAgentName(validationTrigger, "trigger"); err != nil {
		h.respondError(c, http.StatusBadRequest, NewAutomationValidationError(err))
		return
	}
	projectionCtx, ok := h.requireGatewayIngressReadContextForWorkspace(
		c,
		"automation.triggers.create",
		trigger.WorkspaceID,
	)
	if !ok {
		return
	}
	created, err := manager.CreateTrigger(c.Request.Context(), trigger, webhookSecret)
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}

	payload := h.triggerPayloadBestEffort(projectionCtx, created, "automation.triggers.create")
	if err := h.decorateAutomationTriggerOwner(c.Request.Context(), &payload); err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	c.JSON(http.StatusCreated, contract.TriggerResponse{Trigger: payload})
}

// GetAutomationTrigger returns one automation trigger by id.
func (h *BaseHandlers) GetAutomationTrigger(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}
	projectionCtx, ok := h.requireGatewayIngressReadContext(c, "automation.triggers.get")
	if !ok {
		return
	}

	readScope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	trigger, err := manager.GetTrigger(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	if !readScope.Matches(trigger.ProfileID) {
		h.respondError(c, http.StatusNotFound, automationpkg.ErrTriggerNotFound)
		return
	}
	payload, err := h.triggerPayload(projectionCtx, trigger)
	if err != nil {
		h.respondGatewayError(c, err)
		return
	}
	if err := h.decorateAutomationTriggerOwner(c.Request.Context(), &payload); err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.TriggerResponse{Trigger: payload})
}

// UpdateAutomationTrigger patches one automation trigger definition.
func (h *BaseHandlers) UpdateAutomationTrigger(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}
	mutationScope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}

	var req contract.UpdateTriggerRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewAutomationValidationError(
				fmt.Errorf("%s: decode automation trigger update request: %w", h.transportName(), err),
			),
		)
		return
	}
	if !req.HasChanges() {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewAutomationValidationError(errors.New("automation trigger update must include at least one field")),
		)
		return
	}

	current, err := manager.GetTrigger(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	if !mutationScope.Matches(current.ProfileID) {
		h.respondError(c, http.StatusNotFound, automationpkg.ErrTriggerNotFound)
		return
	}
	projectionCtx, ok := h.requireGatewayIngressReadContextForWorkspace(
		c,
		"automation.triggers.update",
		current.WorkspaceID,
	)
	if !ok {
		return
	}

	var updated automationpkg.Trigger
	switch current.Source {
	case automationpkg.JobSourceConfig, automationpkg.JobSourcePackage:
		if err := validateManagedTriggerUpdate(req); err != nil {
			h.respondError(c, http.StatusBadRequest, NewAutomationValidationError(err))
			return
		}
		updated, err = manager.SetTriggerEnabled(c.Request.Context(), current.ID, *req.Enabled)
	default:
		next, patchErr := applyTriggerPatch(current, req)
		if patchErr != nil {
			h.respondError(c, http.StatusBadRequest, NewAutomationValidationError(patchErr))
			return
		}
		if err := automationpkg.ValidateTriggerAgentName(next, "trigger"); err != nil {
			h.respondError(c, http.StatusBadRequest, NewAutomationValidationError(err))
			return
		}
		webhookSecret := webhookSecretWriteFromUpdateRequest(req)
		updated, err = manager.UpdateTrigger(c.Request.Context(), next, webhookSecret)
	}
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}

	payload := h.triggerPayloadBestEffort(projectionCtx, updated, "automation.triggers.update")
	if err := h.decorateAutomationTriggerOwner(c.Request.Context(), &payload); err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.TriggerResponse{Trigger: payload})
}

// DeleteAutomationTrigger removes one dynamic automation trigger definition.
func (h *BaseHandlers) DeleteAutomationTrigger(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}
	mutationScope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}

	current, err := manager.GetTrigger(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	if !mutationScope.Matches(current.ProfileID) {
		h.respondError(c, http.StatusNotFound, automationpkg.ErrTriggerNotFound)
		return
	}
	if _, ok := h.requireGatewayIngressReadContextForWorkspace(
		c,
		"automation.triggers.delete",
		current.WorkspaceID,
	); !ok {
		return
	}
	if current.Source != automationpkg.JobSourceDynamic {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewAutomationValidationError(errors.New("managed automation triggers cannot be deleted")),
		)
		return
	}

	if err := manager.DeleteTrigger(c.Request.Context(), current.ID); err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	c.Status(http.StatusNoContent)
}

// AutomationTriggerRuns returns run history for one automation trigger.
func (h *BaseHandlers) AutomationTriggerRuns(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}

	readScope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	trigger, err := manager.GetTrigger(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	if !readScope.Matches(trigger.ProfileID) {
		h.respondError(c, http.StatusNotFound, automationpkg.ErrTriggerNotFound)
		return
	}

	query, err := ParseAutomationRunQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, NewAutomationValidationError(err))
		return
	}
	query.JobID = ""
	query.TriggerID = trigger.ID
	query.ReadScope = readScope

	runs, err := manager.ListRuns(c.Request.Context(), query)
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	runPayloads := RunPayloadsFromRuns(runs)
	if err := h.decorateAutomationRunOwners(c.Request.Context(), runPayloads); err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.RunsResponse{Runs: runPayloads})
}

// ListAutomationRuns returns filtered automation run history.
func (h *BaseHandlers) ListAutomationRuns(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}

	readScope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	query, err := ParseAutomationRunQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, NewAutomationValidationError(err))
		return
	}
	query.ReadScope = readScope

	runs, err := manager.ListRuns(c.Request.Context(), query)
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	runPayloads := RunPayloadsFromRuns(runs)
	if err := h.decorateAutomationRunOwners(c.Request.Context(), runPayloads); err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.RunsResponse{Runs: runPayloads})
}

// DeliverGlobalWebhook handles external webhook delivery for global triggers.
func (h *BaseHandlers) DeliverGlobalWebhook(c *gin.Context) {
	h.deliverWebhook(c, automationpkg.AutomationScopeGlobal)
}

// DeliverWorkspaceWebhook handles external webhook delivery for workspace-scoped triggers.
func (h *BaseHandlers) DeliverWorkspaceWebhook(c *gin.Context) {
	h.deliverWebhook(c, automationpkg.AutomationScopeWorkspace)
}

func (h *BaseHandlers) deliverWebhook(c *gin.Context, scope automationpkg.Scope) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}

	request, err := webhookRequestFromHTTP(c, scope)
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}

	result, err := manager.HandleWebhook(c.Request.Context(), request)
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.WebhookDeliveryResponse{Result: WebhookDeliveryPayloadFromResult(result)})
}

func (h *BaseHandlers) requireAutomationManager(c *gin.Context) (AutomationManager, bool) {
	if h.Automation == nil {
		h.respondError(
			c,
			http.StatusServiceUnavailable,
			fmt.Errorf("%s: automation manager is not configured", h.transportName()),
		)
		return nil, false
	}
	return h.Automation, true
}

func (h *BaseHandlers) automationSchedulerStateByJobID(
	ctx context.Context,
	manager AutomationManager,
) (map[string]contract.AutomationSchedulerStatePayload, error) {
	if manager == nil {
		return nil, nil
	}

	status, err := manager.Status(ctx)
	if err != nil {
		return nil, err
	}

	stateByID := make(map[string]contract.AutomationSchedulerStatePayload, len(status.ScheduledJobs))
	for _, scheduled := range status.ScheduledJobs {
		stateByID[scheduled.JobID] = AutomationSchedulerStatePayloadFromState(scheduled)
	}
	return stateByID, nil
}

func (h *BaseHandlers) automationSchedulerStateByJobIDBestEffort(
	ctx context.Context,
	manager AutomationManager,
	operation string,
) map[string]contract.AutomationSchedulerStatePayload {
	stateByID, err := h.automationSchedulerStateByJobID(ctx, manager)
	if err == nil {
		return stateByID
	}

	h.Logger.Warn(
		"api: automation scheduler state enrichment failed",
		"transport", h.transportName(),
		handlersOperationKey, strings.TrimSpace(operation),
		"error", err,
	)
	return nil
}

func schedulerStatePointerFromMap(
	states map[string]contract.AutomationSchedulerStatePayload,
	jobID string,
) *contract.AutomationSchedulerStatePayload {
	if states == nil {
		return nil
	}
	state, ok := states[jobID]
	if !ok {
		return nil
	}
	return &state
}

func schedulerNextRunFromMap(
	states map[string]contract.AutomationSchedulerStatePayload,
	jobID string,
) *time.Time {
	return schedulerNextRun(schedulerStatePointerFromMap(states, jobID))
}

func (h *BaseHandlers) automationHealth(ctx context.Context) (contract.AutomationHealthPayload, error) {
	if h.Automation == nil {
		return contract.AutomationHealthPayload{
			Enabled:          false,
			Jobs:             contract.AutomationResourceStatusPayload{},
			Triggers:         contract.AutomationResourceStatusPayload{},
			SchedulerRunning: false,
		}, nil
	}

	status, err := h.Automation.Status(ctx)
	if err != nil {
		return contract.AutomationHealthPayload{}, err
	}
	return AutomationHealthPayloadFromStatus(h.Config.Automation.Enabled, status), nil
}
