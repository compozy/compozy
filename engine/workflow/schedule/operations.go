package schedule

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/robfig/cron/v3"
	"go.temporal.io/sdk/client"
)

// Precompiled 6-field cron parser to avoid repeated allocations
var cronParser6Fields = cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// EnsureTemporalCron ensures the cron expression is in Temporal's expected format.
// Temporal requires 7 fields for seconds support, so we append year if needed.
// Returns the converted cron expression.
func EnsureTemporalCron(cronExpr string) string {
	// @every syntax is supported by Temporal directly
	if strings.HasPrefix(cronExpr, "@every ") {
		return cronExpr
	}
	// Convert cron expression to 7-field format for Temporal
	fields := strings.Fields(cronExpr)
	switch len(fields) {
	case 5:
		// Standard 5-field cron (minute hour day month weekday) - add seconds and year
		return "0 " + cronExpr + " *"
	case 6:
		// 6-field cron with seconds - add year
		return cronExpr + " *"
	case 7:
		// Already in Temporal format
		return cronExpr
	default:
		// Return as-is, let Temporal handle the validation error
		return cronExpr
	}
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

	// Default timezone
	DefaultTimezone = "UTC"
)

// isValidYearField validates the year field in a 7-field cron expression
func isValidYearField(yearField string) bool {
	// Allow wildcard
	if yearField == "*" {
		return true
	}
	// Handle step values (e.g., */2, 2024-2030/2)
	if strings.Contains(yearField, "/") {
		return isValidStepYearField(yearField)
	}
	// Handle ranges (e.g., 2024-2030)
	if strings.Contains(yearField, "-") {
		return isValidRangeYearField(yearField)
	}
	// Handle comma-separated values (e.g., 2024,2025,2026)
	if strings.Contains(yearField, ",") {
		return isValidListYearField(yearField)
	}
	// Handle single year
	return isValidSingleYear(yearField)
}

// isValidSingleYear validates a single year value
func isValidSingleYear(yearStr string) bool {
	matched, err := regexp.MatchString(`^\d{4}$`, yearStr)
	if err != nil || !matched {
		return false
	}
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		return false
	}
	return year >= 1970 && year <= 3000
}

// isValidRangeYearField validates year ranges (e.g., 2024-2030)
func isValidRangeYearField(yearField string) bool {
	matched, err := regexp.MatchString(`^\d{4}-\d{4}$`, yearField)
	if err != nil || !matched {
		return false
	}
	parts := strings.Split(yearField, "-")
	startYear, err1 := strconv.Atoi(parts[0])
	endYear, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return false
	}
	return startYear >= 1970 && endYear <= 3000 && startYear <= endYear
}

// isValidListYearField validates comma-separated years (e.g., 2024,2025,2026)
func isValidListYearField(yearField string) bool {
	matched, err := regexp.MatchString(`^\d{4}(,\d{4})*$`, yearField)
	if err != nil || !matched {
		return false
	}
	years := strings.SplitSeq(yearField, ",")
	for yearStr := range years {
		if !isValidSingleYear(yearStr) {
			return false
		}
	}
	return true
}

// isValidStepYearField validates step values (e.g., */2, 2024-2030/2)
func isValidStepYearField(yearField string) bool {
	parts := strings.Split(yearField, "/")
	if len(parts) != 2 {
		return false
	}
	// Validate step value
	step, err := strconv.Atoi(parts[1])
	if err != nil || step <= 0 {
		return false
	}
	// Validate base pattern
	base := parts[0]
	if base == "*" {
		return true
	}
	// Recursively validate the base part without step
	return isValidYearField(base)
}

// listSchedulesByPrefix lists all schedules with the given prefix
// REQUIRES: Temporal Advanced Visibility to be enabled for Query functionality
// The Query parameter uses Temporal's search attributes which requires Advanced Visibility
// to filter schedules by ScheduleId prefix efficiently
func (m *manager) listSchedulesByPrefix(ctx context.Context, prefix string) (map[string]client.ScheduleHandle, error) {
	log := logger.FromContext(ctx)
	schedules := make(map[string]client.ScheduleHandle)
	// Create iterator for listing schedules
	iter, err := m.client.ScheduleClient().List(ctx, client.ScheduleListOptions{
		PageSize: m.config.PageSize,
		Query:    fmt.Sprintf("ScheduleId STARTS_WITH %q", prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create schedule iterator: %w", err)
	}
	// Iterate through all schedules with error tracking to prevent infinite loops
	const maxConsecutiveErrors = 5
	consecutiveErrors := 0
	for iter.HasNext() {
		schedule, err := iter.Next()
		if err != nil {
			consecutiveErrors++
			log.Warn("Failed to retrieve schedule from iterator",
				"error", err,
				"consecutive_errors", consecutiveErrors)
			// Break if we hit too many consecutive errors to prevent infinite loops
			if consecutiveErrors >= maxConsecutiveErrors {
				log.Error("Too many consecutive iterator errors, aborting iteration",
					"consecutive_errors", consecutiveErrors,
					"schedules_found", len(schedules))
				break
			}
			continue
		}
		// Reset consecutive error counter on successful iteration
		consecutiveErrors = 0
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
		// Use the stored original schedule if available
		if override.OriginalSchedule != nil {
			info.YAMLConfig = override.OriginalSchedule
		} else {
			// Fallback to reconstructing from values for backwards compatibility
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
			if jitterVal, ok := override.Values["original_jitter"].(string); ok {
				info.YAMLConfig.Jitter = jitterVal
			}
			if overlapPolicyVal, ok := override.Values["original_overlap_policy"].(string); ok {
				info.YAMLConfig.OverlapPolicy = workflow.OverlapPolicy(overlapPolicyVal)
			}
			if startAtVal, ok := override.Values["original_start_at"].(time.Time); ok {
				info.YAMLConfig.StartAt = &startAtVal
			}
			if endAtVal, ok := override.Values["original_end_at"].(time.Time); ok {
				info.YAMLConfig.EndAt = &endAtVal
			}
			if inputVal, ok := override.Values["original_input"].(map[string]any); ok {
				info.YAMLConfig.Input = inputVal
			}
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

// ValidateCronExpression validates a cron expression with optional logging context
// Compozy supports two formats:
//  1. @every syntax for intervals: "@every 15s", "@every 1h30m"
//  2. 6-field cron expressions with seconds:
//     Format: "second minute hour day-of-month month day-of-week"
//     Example: "0 0 9 * * 1-5" (Every weekday at 9:00:00 AM)
//     Note: When sending to Temporal, we automatically append year field
func ValidateCronExpression(ctx context.Context, cronExpr string, workflowID string) error {
	log := logger.FromContext(ctx)
	// Check if it's an @every expression
	if after, ok := strings.CutPrefix(cronExpr, "@every "); ok {
		durationStr := after
		_, err := time.ParseDuration(durationStr)
		if err != nil {
			log.Error("Schedule validation failed - invalid @every duration",
				"workflow_id", workflowID,
				"cron", cronExpr,
				"error", err)
			return fmt.Errorf("invalid @every duration '%s': %w", durationStr, err)
		}
		return nil
	}
	// Handle standard macros
	switch cronExpr {
	case "@yearly", "@annually":
		cronExpr = "0 0 0 1 1 *" // sec min hour dom month dow
	case "@monthly":
		cronExpr = "0 0 0 1 * *"
	case "@weekly":
		cronExpr = "0 0 0 * * 0"
	case "@daily", "@midnight":
		cronExpr = "0 0 0 * * *"
	case "@hourly":
		cronExpr = "0 0 * * * *"
	}
	// Parse as 6 or 7-field cron expression
	fields := len(strings.Fields(cronExpr))
	switch fields {
	case 6:
		// 6-field format: second minute hour day month weekday
		if _, err := cronParser6Fields.Parse(cronExpr); err != nil {
			log.Error("Schedule validation failed",
				"workflow_id", workflowID,
				"error", err,
				"cron", cronExpr)
			return fmt.Errorf("invalid 6-field cron expression '%s': %w", cronExpr, err)
		}
	case 7:
		// 7-field format: second minute hour day month weekday year
		// Parse first 6 fields with standard parser
		cronFields := strings.Fields(cronExpr)
		sixFields := strings.Join(cronFields[:6], " ")
		if _, err := cronParser6Fields.Parse(sixFields); err != nil {
			log.Error("Schedule validation failed",
				"workflow_id", workflowID,
				"error", err,
				"cron", cronExpr)
			return fmt.Errorf("invalid 7-field cron expression '%s': %w", cronExpr, err)
		}
		// Validate year field (7th field) - should be * or a valid year range
		yearField := cronFields[6]
		if !isValidYearField(yearField) {
			log.Error("Schedule validation failed - invalid year field",
				"workflow_id", workflowID,
				"cron", cronExpr,
				"year_field", yearField)
			return fmt.Errorf("invalid year field in cron expression '%s': %s", cronExpr, yearField)
		}
	default:
		log.Error("Schedule validation failed - incorrect field count",
			"workflow_id", workflowID,
			"cron", cronExpr,
			"expected_fields", "6 or 7",
			"actual_fields", fields)
		return fmt.Errorf(
			"invalid cron expression '%s': expected 6 or 7 fields "+
				"(second minute hour day month weekday [year]), got %d fields",
			cronExpr,
			fields,
		)
	}
	return nil
}

// validateCronForSchedule validates cron expression for schedule creation
func (m *manager) validateCronForSchedule(ctx context.Context, wf *workflow.Config) error {
	return ValidateCronExpression(ctx, wf.Schedule.Cron, wf.ID)
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
	if err := m.validateCronForSchedule(ctx, wf); err != nil {
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
		spec.TimeZoneName = DefaultTimezone
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
		Workflow:                 worker.CompozyWorkflowName, // Use constant workflow type name
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
	// Validate cron expression before attempting update
	if err := m.validateCronForSchedule(ctx, wf); err != nil {
		return err
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
				action.Workflow = worker.CompozyWorkflowName
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

	// Check core schedule properties
	needsUpdate = m.checkCronNeedsUpdate(currentSpec, wf) ||
		m.checkTimezoneNeedsUpdate(currentSpec, wf) ||
		m.checkEnabledStateNeedsUpdate(desc, wf) ||
		m.checkJitterNeedsUpdate(currentSpec, wf) ||
		m.checkStartEndTimesNeedsUpdate(currentSpec, wf)

	// Set expected values for return
	expectedTimezone = wf.Schedule.Timezone
	if expectedTimezone == "" {
		expectedTimezone = DefaultTimezone
	}
	expectedEnabled = true
	if wf.Schedule.Enabled != nil {
		expectedEnabled = *wf.Schedule.Enabled
	}

	return needsUpdate, expectedTimezone, expectedEnabled
}

// checkCronNeedsUpdate checks if cron expression needs updating
func (m *manager) checkCronNeedsUpdate(currentSpec *client.ScheduleSpec, wf *workflow.Config) bool {
	expectedCron := EnsureTemporalCron(wf.Schedule.Cron)
	return len(currentSpec.CronExpressions) == 0 || currentSpec.CronExpressions[0] != expectedCron
}

// checkTimezoneNeedsUpdate checks if timezone needs updating
func (m *manager) checkTimezoneNeedsUpdate(currentSpec *client.ScheduleSpec, wf *workflow.Config) bool {
	expectedTimezone := wf.Schedule.Timezone
	if expectedTimezone == "" {
		expectedTimezone = DefaultTimezone
	}
	// Treat empty timezone as UTC to handle schedules created before defaults were enforced
	currentTimezone := currentSpec.TimeZoneName
	if currentTimezone == "" {
		currentTimezone = DefaultTimezone
	}
	return currentTimezone != expectedTimezone
}

// checkEnabledStateNeedsUpdate checks if enabled state needs updating
func (m *manager) checkEnabledStateNeedsUpdate(desc *client.ScheduleDescription, wf *workflow.Config) bool {
	expectedEnabled := true
	if wf.Schedule.Enabled != nil {
		expectedEnabled = *wf.Schedule.Enabled
	}
	isCurrentlyPaused := desc.Schedule.State.Paused
	shouldBePaused := !expectedEnabled
	return isCurrentlyPaused != shouldBePaused
}

// checkJitterNeedsUpdate checks if jitter needs updating
func (m *manager) checkJitterNeedsUpdate(currentSpec *client.ScheduleSpec, wf *workflow.Config) bool {
	expectedJitter := time.Duration(0)
	if wf.Schedule.Jitter != "" {
		if jitter, err := time.ParseDuration(wf.Schedule.Jitter); err == nil {
			expectedJitter = jitter
		}
	}
	return currentSpec.Jitter != expectedJitter
}

// checkStartEndTimesNeedsUpdate checks if start/end times need updating
func (m *manager) checkStartEndTimesNeedsUpdate(currentSpec *client.ScheduleSpec, wf *workflow.Config) bool {
	// Check start time
	if (wf.Schedule.StartAt == nil && !currentSpec.StartAt.IsZero()) ||
		(wf.Schedule.StartAt != nil && !currentSpec.StartAt.Equal(*wf.Schedule.StartAt)) {
		return true
	}
	// Check end time
	if (wf.Schedule.EndAt == nil && !currentSpec.EndAt.IsZero()) ||
		(wf.Schedule.EndAt != nil && !currentSpec.EndAt.Equal(*wf.Schedule.EndAt)) {
		return true
	}
	return false
}
