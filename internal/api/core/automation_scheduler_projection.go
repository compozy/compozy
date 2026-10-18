package core

import (
	"context"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
)

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
