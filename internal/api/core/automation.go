package core

import (
	"errors"
	"fmt"

	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	automationpkg "github.com/compozy/compozy/internal/automation"
	"github.com/gin-gonic/gin"
)

const (
	// WebhookTimestampHeader is the required HTTP header that carries the
	// signed webhook timestamp.
	WebhookTimestampHeader = "X-Compozy-Webhook-Timestamp"
	// WebhookSignatureHeader is the required HTTP header that carries the HMAC
	// signature for webhook delivery.
	WebhookSignatureHeader = "X-Compozy-Webhook-Signature"
	// WebhookDeliveryIDHeader identifies one webhook delivery so replayed
	// requests can be rejected inside the trigger engine.
	WebhookDeliveryIDHeader = "X-Compozy-Webhook-Delivery-ID"

	maxWebhookPayloadSize = 1 << 20
)

// ListAutomationJobs returns the filtered automation job list.
func (h *BaseHandlers) ListAutomationJobs(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}

	readScope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	query, err := ParseAutomationJobListQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, NewAutomationValidationError(err))
		return
	}
	query.ReadScope = readScope

	page, err := manager.ListJobs(c.Request.Context(), query)
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}

	schedulerStateByID, err := h.automationSchedulerStateByJobID(c.Request.Context(), manager)
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}

	jobPayloads := JobPayloadsFromJobs(page.Jobs, schedulerStateByID)
	if err := h.decorateAutomationJobOwners(c.Request.Context(), jobPayloads); err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.JobsResponse{
		Jobs: jobPayloads,
		Page: contract.CountedCursorPagePayload{
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
			Total:      page.Total,
			Limit:      page.Limit,
		},
	})
}

// CreateAutomationJob stores a new dynamic automation job.
func (h *BaseHandlers) CreateAutomationJob(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}
	mutationScope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}

	var req contract.CreateJobRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewAutomationValidationError(
				fmt.Errorf("%s: decode automation job create request: %w", h.transportName(), err),
			),
		)
		return
	}

	job, err := jobFromCreateRequest(req)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, NewAutomationValidationError(err))
		return
	}
	job.ProfileID = mutationScope.ProfileID
	if err := job.Validate("job"); err != nil {
		h.respondError(c, http.StatusBadRequest, NewAutomationValidationError(err))
		return
	}
	if err := automationpkg.ValidateJobAgentName(job, "job"); err != nil {
		h.respondError(c, http.StatusBadRequest, NewAutomationValidationError(err))
		return
	}

	created, err := manager.CreateJob(c.Request.Context(), job)
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}

	schedulerStateByID := h.automationSchedulerStateByJobIDBestEffort(c.Request.Context(), manager, "create_job")

	payload := JobPayloadFromJob(
		created,
		schedulerNextRunFromMap(schedulerStateByID, created.ID),
		schedulerStatePointerFromMap(schedulerStateByID, created.ID),
	)
	if err := h.decorateAutomationJobOwner(c.Request.Context(), &payload); err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	c.JSON(http.StatusCreated, contract.JobResponse{Job: payload})
}

// GetAutomationJob returns one automation job by id.
func (h *BaseHandlers) GetAutomationJob(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}

	readScope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	job, err := manager.GetJob(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	if !readScope.Matches(job.ProfileID) {
		h.respondError(c, http.StatusNotFound, automationpkg.ErrJobNotFound)
		return
	}

	schedulerStateByID, err := h.automationSchedulerStateByJobID(c.Request.Context(), manager)
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}

	payload := JobPayloadFromJob(
		job,
		schedulerNextRunFromMap(schedulerStateByID, job.ID),
		schedulerStatePointerFromMap(schedulerStateByID, job.ID),
	)
	if err := h.decorateAutomationJobOwner(c.Request.Context(), &payload); err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.JobResponse{Job: payload})
}

// UpdateAutomationJob patches one automation job definition.
func (h *BaseHandlers) UpdateAutomationJob(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}
	mutationScope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}

	var req contract.UpdateJobRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewAutomationValidationError(
				fmt.Errorf("%s: decode automation job update request: %w", h.transportName(), err),
			),
		)
		return
	}
	if !req.HasChanges() {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewAutomationValidationError(errors.New("automation job update must include at least one field")),
		)
		return
	}

	current, err := manager.GetJob(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	if !mutationScope.Matches(current.ProfileID) {
		h.respondError(c, http.StatusNotFound, automationpkg.ErrJobNotFound)
		return
	}

	updated, err := updateAutomationJob(c.Request.Context(), manager, current, req)
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}

	schedulerStateByID := h.automationSchedulerStateByJobIDBestEffort(c.Request.Context(), manager, "update_job")

	payload := JobPayloadFromJob(
		updated,
		schedulerNextRunFromMap(schedulerStateByID, updated.ID),
		schedulerStatePointerFromMap(schedulerStateByID, updated.ID),
	)
	if err := h.decorateAutomationJobOwner(c.Request.Context(), &payload); err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.JobResponse{Job: payload})
}

// DeleteAutomationJob removes one dynamic automation job definition.
func (h *BaseHandlers) DeleteAutomationJob(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}
	mutationScope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}

	current, err := manager.GetJob(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	if !mutationScope.Matches(current.ProfileID) {
		h.respondError(c, http.StatusNotFound, automationpkg.ErrJobNotFound)
		return
	}
	if current.Source != automationpkg.JobSourceDynamic {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewAutomationValidationError(errors.New("managed automation jobs cannot be deleted")),
		)
		return
	}

	if err := manager.DeleteJob(c.Request.Context(), current.ID); err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	c.Status(http.StatusNoContent)
}

// TriggerAutomationJob forces one immediate manual automation run.
func (h *BaseHandlers) TriggerAutomationJob(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}
	mutationScope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	job, err := manager.GetJob(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	if !mutationScope.Matches(job.ProfileID) {
		h.respondError(c, http.StatusNotFound, automationpkg.ErrJobNotFound)
		return
	}

	run, err := manager.TriggerJob(c.Request.Context(), job.ID)
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	run.ProfileID = job.ProfileID
	payload := RunPayloadFromRun(run)
	if err := h.decorateAutomationRunOwner(c.Request.Context(), &payload); err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.RunResponse{Run: payload})
}

// AutomationJobRuns returns run history for one automation job.
func (h *BaseHandlers) AutomationJobRuns(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}

	readScope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	job, err := manager.GetJob(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	if !readScope.Matches(job.ProfileID) {
		h.respondError(c, http.StatusNotFound, automationpkg.ErrJobNotFound)
		return
	}

	query, err := ParseAutomationRunQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, NewAutomationValidationError(err))
		return
	}
	query.JobID = job.ID
	query.TriggerID = ""
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
