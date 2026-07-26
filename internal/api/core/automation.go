package core

import (
	"errors"
	"fmt"

	"net/http"

	"github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"
	"github.com/gin-gonic/gin"
)

const (
	// WebhookTimestampHeader is the required HTTP header that carries the
	// signed webhook timestamp.
	WebhookTimestampHeader = "X-AGH-Webhook-Timestamp"
	// WebhookSignatureHeader is the required HTTP header that carries the HMAC
	// signature for webhook delivery.
	WebhookSignatureHeader = "X-AGH-Webhook-Signature"
	// WebhookDeliveryIDHeader identifies one webhook delivery so replayed
	// requests can be rejected inside the trigger engine.
	WebhookDeliveryIDHeader = "X-AGH-Webhook-Delivery-ID"

	maxWebhookPayloadSize = 1 << 20
)

// ListAutomationJobs returns the filtered automation job list.
func (h *BaseHandlers) ListAutomationJobs(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}

	query, err := ParseAutomationJobListQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, NewAutomationValidationError(err))
		return
	}

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

	c.JSON(http.StatusOK, contract.JobsResponse{
		Jobs: JobPayloadsFromJobs(page.Jobs, schedulerStateByID),
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
	if err := job.Validate("job"); err != nil {
		h.respondError(c, http.StatusBadRequest, NewAutomationValidationError(err))
		return
	}

	created, err := manager.CreateJob(c.Request.Context(), job)
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}

	schedulerStateByID := h.automationSchedulerStateByJobIDBestEffort(c.Request.Context(), manager, "create_job")

	c.JSON(
		http.StatusCreated,
		contract.JobResponse{Job: JobPayloadFromJob(
			created,
			schedulerNextRunFromMap(schedulerStateByID, created.ID),
			schedulerStatePointerFromMap(schedulerStateByID, created.ID),
		)},
	)
}

// GetAutomationJob returns one automation job by id.
func (h *BaseHandlers) GetAutomationJob(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}

	job, err := manager.GetJob(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}

	schedulerStateByID, err := h.automationSchedulerStateByJobID(c.Request.Context(), manager)
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.JobResponse{Job: JobPayloadFromJob(
		job,
		schedulerNextRunFromMap(schedulerStateByID, job.ID),
		schedulerStatePointerFromMap(schedulerStateByID, job.ID),
	)})
}

// UpdateAutomationJob patches one automation job definition.
func (h *BaseHandlers) UpdateAutomationJob(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
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

	var updated automationpkg.Job
	switch current.Source {
	case automationpkg.JobSourceConfig, automationpkg.JobSourcePackage:
		if err := validateManagedJobUpdate(req); err != nil {
			h.respondError(c, http.StatusBadRequest, NewAutomationValidationError(err))
			return
		}
		updated, err = manager.SetJobEnabled(c.Request.Context(), current.ID, *req.Enabled)
	default:
		next, patchErr := applyJobPatch(current, req)
		if patchErr != nil {
			h.respondError(c, http.StatusBadRequest, NewAutomationValidationError(patchErr))
			return
		}
		if err := next.Validate("job"); err != nil {
			h.respondError(c, http.StatusBadRequest, NewAutomationValidationError(err))
			return
		}
		updated, err = manager.UpdateJob(c.Request.Context(), next)
	}
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}

	schedulerStateByID := h.automationSchedulerStateByJobIDBestEffort(c.Request.Context(), manager, "update_job")

	c.JSON(
		http.StatusOK,
		contract.JobResponse{Job: JobPayloadFromJob(
			updated,
			schedulerNextRunFromMap(schedulerStateByID, updated.ID),
			schedulerStatePointerFromMap(schedulerStateByID, updated.ID),
		)},
	)
}

// DeleteAutomationJob removes one dynamic automation job definition.
func (h *BaseHandlers) DeleteAutomationJob(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}

	current, err := manager.GetJob(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
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

	run, err := manager.TriggerJob(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.RunResponse{Run: RunPayloadFromRun(run)})
}

// AutomationJobRuns returns run history for one automation job.
func (h *BaseHandlers) AutomationJobRuns(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}

	job, err := manager.GetJob(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}

	query, err := ParseAutomationRunQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, NewAutomationValidationError(err))
		return
	}
	query.JobID = job.ID
	query.TriggerID = ""

	runs, err := manager.ListRuns(c.Request.Context(), query)
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.RunsResponse{Runs: RunPayloadsFromRuns(runs)})
}
