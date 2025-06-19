package schedule

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/robfig/cron/v3"
	"go.temporal.io/sdk/client"
)

// Operation status constants for metrics
const (
	OperationStatusSuccess = "success"
	OperationStatusFailure = "failure"
)

// listSchedulesByPrefix lists all schedules with the given prefix
func (m *manager) listSchedulesByPrefix(ctx context.Context, prefix string) (map[string]client.ScheduleHandle, error) {
	log := logger.FromContext(ctx)
	schedules := make(map[string]client.ScheduleHandle)
	// Create iterator for listing schedules
	iter, err := m.client.ScheduleClient().List(ctx, client.ScheduleListOptions{
		PageSize: 100,
		Query:    fmt.Sprintf("ScheduleId STARTS_WITH %q", prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create schedule iterator: %w", err)
	}
	// Iterate through all schedules
	for iter.HasNext() {
		schedule, err := iter.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to list schedules: %w", err)
		}
		scheduleID := schedule.ID
		handle := m.client.ScheduleClient().GetHandle(ctx, scheduleID)
		schedules[scheduleID] = handle
		log.Debug("Found schedule", "schedule_id", scheduleID)
	}
	return schedules, nil
}

// getScheduleInfo retrieves detailed information about a schedule
func (m *manager) getScheduleInfo(
	ctx context.Context,
	scheduleID string,
	handle client.ScheduleHandle,
) (*Info, error) {
	desc, err := handle.Describe(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to describe schedule: %w", err)
	}
	workflowID := m.workflowIDFromScheduleID(scheduleID)
	// Extract schedule information
	spec := desc.Schedule.Spec
	state := desc.Schedule.State
	info := &Info{
		WorkflowID: workflowID,
		ScheduleID: scheduleID,
		Enabled:    !state.Paused,
		Timezone:   spec.TimeZoneName,
	}
	// Extract cron expression from calendar specs
	if len(spec.CronExpressions) > 0 {
		info.Cron = spec.CronExpressions[0]
	}

	// Check for API overrides and populate YAMLConfig
	override, hasOverride := m.overrideCache.GetOverride(workflowID)
	if hasOverride {
		info.IsOverride = true
		// Create YAMLConfig from override values if available
		info.YAMLConfig = &workflow.Schedule{}

		// Set original YAML values from override cache or defaults
		if cronVal, ok := override.Values["original_cron"].(string); ok {
			info.YAMLConfig.Cron = cronVal
		}
		if enabledVal, ok := override.Values["original_enabled"].(bool); ok {
			info.YAMLConfig.Enabled = &enabledVal
		}
		if timezoneVal, ok := override.Values["original_timezone"].(string); ok {
			info.YAMLConfig.Timezone = timezoneVal
		}
	}

	// Set run times
	if len(desc.Info.NextActionTimes) > 0 {
		info.NextRunTime = desc.Info.NextActionTimes[0]
	}
	if len(desc.Info.RecentActions) > 0 {
		lastAction := desc.Info.RecentActions[0]
		info.LastRunTime = &lastAction.ScheduleTime
		// Set default status since Action field is not available in SDK
		info.LastRunStatus = "unknown"
	}
	return info, nil
}

// createSchedule creates a new schedule in Temporal
func (m *manager) createSchedule(ctx context.Context, scheduleID string, wf *workflow.Config) error {
	log := logger.FromContext(ctx).With("schedule_id", scheduleID, "workflow_id", wf.ID)

	// Record operation metrics
	operation := "create"
	status := OperationStatusFailure // Assume failure, will be set to success on completion
	if m.metrics != nil {
		defer func() {
			m.metrics.RecordOperation(ctx, operation, status, m.projectID)
		}()
	}
	// Parse cron expression
	cronSpec, err := cron.ParseStandard(wf.Schedule.Cron)
	if err != nil {
		log.Error("Schedule validation failed",
			"workflow_id", wf.ID,
			"error", err,
			"cron", wf.Schedule.Cron)
		return fmt.Errorf("invalid cron expression: %w", err)
	}
	_ = cronSpec // Just for validation
	// Build schedule specification
	spec := client.ScheduleSpec{
		CronExpressions: []string{wf.Schedule.Cron},
	}
	// Set timezone
	if wf.Schedule.Timezone != "" {
		spec.TimeZoneName = wf.Schedule.Timezone
	} else {
		spec.TimeZoneName = "UTC"
	}
	// Handle jitter
	if wf.Schedule.Jitter != "" {
		jitterDuration, err := time.ParseDuration(wf.Schedule.Jitter)
		if err != nil {
			log.Error("Schedule validation failed",
				"workflow_id", wf.ID,
				"error", err,
				"jitter", wf.Schedule.Jitter)
			return fmt.Errorf("invalid jitter duration: %w", err)
		}
		spec.Jitter = jitterDuration
	}
	// Handle start and end times
	if wf.Schedule.StartAt != nil {
		spec.StartAt = *wf.Schedule.StartAt
	}
	if wf.Schedule.EndAt != nil {
		spec.EndAt = *wf.Schedule.EndAt
	}
	// Build schedule action (workflow to run)
	action := &client.ScheduleWorkflowAction{
		ID:                       wf.ID,
		Workflow:                 wf.ID, // Workflow type name
		TaskQueue:                m.taskQueue,
		WorkflowExecutionTimeout: 0, // Use server default
		WorkflowRunTimeout:       0, // Use server default
		WorkflowTaskTimeout:      0, // Use server default
	}
	// Set workflow input
	if wf.Schedule.Input != nil {
		action.Args = []any{wf.Schedule.Input}
	}
	// Note: Overlap policy is handled differently in SDK v1.34.0
	// It's set during trigger operations rather than during creation
	if wf.Schedule.OverlapPolicy != "" && wf.Schedule.OverlapPolicy != workflow.OverlapSkip {
		log.Warn("OverlapPolicy is configured but not enforced by schedule creation in this SDK version",
			"workflow_id", wf.ID,
			"policy", wf.Schedule.OverlapPolicy,
			"info", "Policy must be handled by the workflow or custom trigger logic")
	}
	// Create schedule state
	state := client.ScheduleState{
		Paused: false, // Default to enabled
	}
	if wf.Schedule.Enabled != nil && !*wf.Schedule.Enabled {
		state.Paused = true
	}
	// Create schedule options - simplified version for SDK v1.34.0
	options := client.ScheduleOptions{
		ID:     scheduleID,
		Spec:   spec,
		Action: action,
		Paused: state.Paused,
		Memo: map[string]any{
			"project_id":  m.projectID,
			"workflow_id": wf.ID,
		},
	}
	// Create the schedule
	handle, err := m.client.ScheduleClient().Create(ctx, options)
	if err != nil {
		return fmt.Errorf("failed to create schedule: %w", err)
	}
	status = OperationStatusSuccess // Mark operation as successful
	log.Info("Schedule created successfully", "handle", handle.GetID())
	return nil
}

// updateSchedule updates an existing schedule in Temporal
func (m *manager) updateSchedule(ctx context.Context, scheduleID string, wf *workflow.Config) error {
	log := logger.FromContext(ctx).With("schedule_id", scheduleID, "workflow_id", wf.ID)

	// Record operation metrics
	operation := "update"
	status := OperationStatusFailure // Assume failure, will be set to success on completion
	if m.metrics != nil {
		defer func() {
			m.metrics.RecordOperation(ctx, operation, status, m.projectID)
		}()
	}
	handle := m.client.ScheduleClient().GetHandle(ctx, scheduleID)
	// Get current description to check if update is needed
	desc, err := handle.Describe(ctx)
	if err != nil {
		return fmt.Errorf("failed to describe schedule for update: %w", err)
	}
	// Check if update is needed
	needsUpdate, expectedTimezone, expectedEnabled := m.checkScheduleNeedsUpdate(desc, wf)
	if !needsUpdate {
		status = OperationStatusSuccess // Mark as successful (no update needed)
		log.Debug("Schedule is up to date, skipping update")
		return nil
	}
	// Perform update
	err = handle.Update(ctx, client.ScheduleUpdateOptions{
		DoUpdate: func(input client.ScheduleUpdateInput) (*client.ScheduleUpdate, error) {
			// Update spec
			input.Description.Schedule.Spec.CronExpressions = []string{wf.Schedule.Cron}
			input.Description.Schedule.Spec.TimeZoneName = expectedTimezone
			// Update jitter
			if wf.Schedule.Jitter != "" {
				jitterDuration, err := time.ParseDuration(wf.Schedule.Jitter)
				if err != nil {
					return nil, fmt.Errorf("invalid jitter duration: %w", err)
				}
				input.Description.Schedule.Spec.Jitter = jitterDuration
			} else {
				input.Description.Schedule.Spec.Jitter = 0
			}
			// Update start/end times
			if wf.Schedule.StartAt != nil {
				input.Description.Schedule.Spec.StartAt = *wf.Schedule.StartAt
			}
			if wf.Schedule.EndAt != nil {
				input.Description.Schedule.Spec.EndAt = *wf.Schedule.EndAt
			}
			// Note: Overlap policy is not updated in SDK v1.34.0
			// Update enabled state
			input.Description.Schedule.State.Paused = !expectedEnabled
			// Update action (workflow input)
			if action, ok := input.Description.Schedule.Action.(*client.ScheduleWorkflowAction); ok {
				if wf.Schedule.Input != nil {
					action.Args = []any{wf.Schedule.Input}
				} else {
					action.Args = nil
				}
			}
			return &client.ScheduleUpdate{
				Schedule: &input.Description.Schedule,
			}, nil
		},
	})
	if err != nil {
		return fmt.Errorf("failed to update schedule: %w", err)
	}
	status = OperationStatusSuccess // Mark operation as successful
	log.Info("Schedule updated successfully")
	return nil
}

// deleteSchedule deletes a schedule from Temporal
func (m *manager) deleteSchedule(ctx context.Context, scheduleID string) error {
	log := logger.FromContext(ctx).With("schedule_id", scheduleID)

	// Record operation metrics
	operation := "delete"
	status := OperationStatusFailure // Assume failure, will be set to success on completion
	if m.metrics != nil {
		defer func() {
			m.metrics.RecordOperation(ctx, operation, status, m.projectID)
		}()
	}
	handle := m.client.ScheduleClient().GetHandle(ctx, scheduleID)
	err := handle.Delete(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete schedule: %w", err)
	}
	// Also remove any overrides associated with the deleted schedule.
	workflowID := m.workflowIDFromScheduleID(scheduleID)
	m.overrideCache.ClearOverride(workflowID)
	status = OperationStatusSuccess // Mark operation as successful
	log.Info("Schedule deleted successfully")
	return nil
}

// checkScheduleNeedsUpdate determines if a schedule needs updating
// Note: This function only compares Temporal state with YAML config.
// Override handling is done at the reconciliation level before this function is called.
func (m *manager) checkScheduleNeedsUpdate(
	desc *client.ScheduleDescription,
	wf *workflow.Config,
) (needsUpdate bool, expectedTimezone string, expectedEnabled bool) {
	currentSpec := desc.Schedule.Spec

	// Check cron expression
	if len(currentSpec.CronExpressions) == 0 || currentSpec.CronExpressions[0] != wf.Schedule.Cron {
		needsUpdate = true
	}

	// Check timezone
	expectedTimezone = wf.Schedule.Timezone
	if expectedTimezone == "" {
		expectedTimezone = "UTC"
	}
	if currentSpec.TimeZoneName != expectedTimezone {
		needsUpdate = true
	}

	// Check enabled state
	expectedEnabled = true
	if wf.Schedule.Enabled != nil {
		expectedEnabled = *wf.Schedule.Enabled
	}
	isCurrentlyPaused := desc.Schedule.State.Paused
	shouldBePaused := !expectedEnabled
	if isCurrentlyPaused != shouldBePaused {
		needsUpdate = true
	}

	return needsUpdate, expectedTimezone, expectedEnabled
}
