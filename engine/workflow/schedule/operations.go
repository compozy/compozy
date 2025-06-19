package schedule

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/robfig/cron/v3"
	"go.temporal.io/sdk/client"
)

// EnsureTemporalCron ensures the cron expression is in Temporal's expected format.
// Temporal requires 7 fields for seconds support, so we append year if needed.
// Returns the converted cron expression.
func EnsureTemporalCron(cronExpr string) string {
	// @every syntax is supported by Temporal directly
	if strings.HasPrefix(cronExpr, "@every ") {
		return cronExpr
	}
	// Convert 6-field to 7-field by adding year
	fields := strings.Fields(cronExpr)
	if len(fields) == 6 {
		return cronExpr + " *"
	}
	return cronExpr
}

// Operation constants for metrics
const (
	// Operation types
	OperationCreate = "create"
	OperationUpdate = "update"
	OperationDelete = "delete"

	// Operation status
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

// validateCronForSchedule validates cron expression for schedule creation
func (m *manager) validateCronForSchedule(wf *workflow.Config, log logger.Logger) error {
	// Compozy supports two formats:
	// 1. @every syntax for intervals: "@every 15s", "@every 1h30m"
	// 2. 6-field cron expressions with seconds:
	//    Format: "second minute hour day-of-month month day-of-week"
	//    Example: "0 0 9 * * 1-5" (Every weekday at 9:00:00 AM)
	//    Note: When sending to Temporal, we automatically append year field

	// Check if it's an @every expression
	if strings.HasPrefix(wf.Schedule.Cron, "@every ") {
		// Extract duration string after "@every "
		durationStr := strings.TrimPrefix(wf.Schedule.Cron, "@every ")
		_, err := time.ParseDuration(durationStr)
		if err != nil {
			log.Error("Schedule validation failed - invalid @every duration",
				"workflow_id", wf.ID,
				"cron", wf.Schedule.Cron,
				"error", err)
			return fmt.Errorf("invalid @every duration '%s': %w", durationStr, err)
		}
		return nil
	}

	// Parse as 6 or 7-field cron expression
	fields := len(strings.Fields(wf.Schedule.Cron))
	var parser cron.Parser
	switch fields {
	case 6:
		// 6-field format: second minute hour day month weekday
		parser = cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	case 7:
		// 7-field format: second minute hour day month weekday year
		parser = cron.NewParser(
			cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
		)
	default:
		log.Error("Schedule validation failed - incorrect field count",
			"workflow_id", wf.ID,
			"cron", wf.Schedule.Cron,
			"expected_fields", "6 or 7",
			"actual_fields", fields)
		return fmt.Errorf(
			"invalid cron expression: expected 6 or 7 fields (second minute hour day month weekday [year]), got %d fields",
			fields,
		)
	}
	_, err := parser.Parse(wf.Schedule.Cron)
	if err != nil {
		log.Error("Schedule validation failed",
			"workflow_id", wf.ID,
			"error", err,
			"cron", wf.Schedule.Cron)
		return fmt.Errorf("invalid cron expression: %w", err)
	}
	return nil
}

// createSchedule creates a new schedule in Temporal
func (m *manager) createSchedule(ctx context.Context, scheduleID string, wf *workflow.Config) error {
	log := logger.FromContext(ctx).With("schedule_id", scheduleID, "workflow_id", wf.ID)

	// Record operation metrics
	operation := OperationCreate
	status := OperationStatusFailure // Assume failure, will be set to success on completion
	if m.metrics != nil {
		defer func() {
			m.metrics.RecordOperation(ctx, operation, status, m.projectID)
		}()
	}
	// Validate cron expression
	if err := m.validateCronForSchedule(wf, log); err != nil {
		return err
	}
	// Build schedule specification
	// Ensure cron expression is in Temporal's expected format
	cronExpr := EnsureTemporalCron(wf.Schedule.Cron)
	spec := client.ScheduleSpec{
		CronExpressions: []string{cronExpr},
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
		Workflow:                 "CompozyWorkflow", // All workflows use the generic CompozyWorkflow type
		TaskQueue:                m.taskQueue,
		WorkflowExecutionTimeout: 0, // Use server default
		WorkflowRunTimeout:       0, // Use server default
		WorkflowTaskTimeout:      0, // Use server default
	}
	// Set workflow input - create TriggerInput for CompozyWorkflow
	triggerInput := map[string]any{
		"workflow_id":      wf.ID,
		"workflow_exec_id": "", // Will be generated by the workflow
		"input":            wf.Schedule.Input,
	}
	action.Args = []any{triggerInput}
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
	operation := OperationUpdate
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
			// Ensure cron expression is in Temporal's expected format
			cronExpr := EnsureTemporalCron(wf.Schedule.Cron)
			input.Description.Schedule.Spec.CronExpressions = []string{cronExpr}
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
			// Update action (workflow input and type)
			if action, ok := input.Description.Schedule.Action.(*client.ScheduleWorkflowAction); ok {
				// Update workflow type to CompozyWorkflow
				action.Workflow = "CompozyWorkflow"
				// Create TriggerInput for CompozyWorkflow
				triggerInput := map[string]any{
					"workflow_id":      wf.ID,
					"workflow_exec_id": "", // Will be generated by the workflow
					"input":            wf.Schedule.Input,
				}
				action.Args = []any{triggerInput}
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
	operation := OperationDelete
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
	// Convert YAML cron to Temporal format for comparison
	expectedCron := EnsureTemporalCron(wf.Schedule.Cron)
	if len(currentSpec.CronExpressions) == 0 || currentSpec.CronExpressions[0] != expectedCron {
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
