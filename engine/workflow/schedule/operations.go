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
	if strings.HasPrefix(cronExpr, "@every ") {
		// NOTE: Temporal accepts @every expressions natively; no conversion needed.
		return cronExpr
	}
	fields := strings.Fields(cronExpr)
	switch len(fields) {
	case 5:
		// NOTE: Pad standard 5-field crons with seconds and year for Temporal compatibility.
		return "0 " + cronExpr + " *"
	case 6:
		// NOTE: Append the year field when seconds are present but year is omitted.
		return cronExpr + " *"
	case 7:
		return cronExpr
	default:
		// NOTE: Leave unexpected formats untouched so Temporal validation surfaces the error.
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
	if yearField == "*" {
		return true
	}
	if strings.Contains(yearField, "/") {
		return isValidStepYearField(yearField)
	}
	if strings.Contains(yearField, "-") {
		return isValidRangeYearField(yearField)
	}
	if strings.Contains(yearField, ",") {
		return isValidListYearField(yearField)
	}
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
	step, err := strconv.Atoi(parts[1])
	if err != nil || step <= 0 {
		return false
	}
	base := parts[0]
	if base == "*" {
		return true
	}
	return isValidYearField(base)
}

// listSchedulesByPrefix lists all schedules with the given prefix
// REQUIRES: Temporal Advanced Visibility to be enabled for Query functionality
// The Query parameter uses Temporal's search attributes which requires Advanced Visibility
// to filter schedules by ScheduleId prefix efficiently
func (m *manager) listSchedulesByPrefix(ctx context.Context, prefix string) (map[string]client.ScheduleHandle, error) {
	log := logger.FromContext(ctx)
	schedules := make(map[string]client.ScheduleHandle)
	iter, err := m.client.ScheduleClient().List(ctx, client.ScheduleListOptions{
		PageSize: m.config.PageSize,
		Query:    fmt.Sprintf("ScheduleId STARTS_WITH %q", prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create schedule iterator: %w", err)
	}
	const maxConsecutiveErrors = 5
	consecutiveErrors := 0
	for iter.HasNext() {
		schedule, err := iter.Next()
		if err != nil {
			consecutiveErrors++
			log.Warn("Failed to retrieve schedule from iterator",
				"error", err,
				"consecutive_errors", consecutiveErrors)
			if consecutiveErrors >= maxConsecutiveErrors {
				log.Error("Too many consecutive iterator errors, aborting iteration",
					"consecutive_errors", consecutiveErrors,
					"schedules_found", len(schedules))
				break
			}
			continue
		}
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
	info := buildBaseScheduleInfo(scheduleID, workflowID, desc)
	m.applyOverrideMetadata(info, workflowID)
	populateScheduleRunTimes(info, desc)
	return info, nil
}

func buildBaseScheduleInfo(
	scheduleID string,
	workflowID string,
	desc *client.ScheduleDescription,
) *Info {
	spec := desc.Schedule.Spec
	state := desc.Schedule.State
	info := &Info{
		WorkflowID: workflowID,
		ScheduleID: scheduleID,
		Enabled:    !state.Paused,
		Timezone:   spec.TimeZoneName,
	}
	if len(spec.CronExpressions) > 0 {
		info.Cron = spec.CronExpressions[0]
	}
	return info
}

func (m *manager) applyOverrideMetadata(info *Info, workflowID string) {
	override, hasOverride := m.overrideCache.GetOverride(workflowID)
	if !hasOverride {
		return
	}
	info.IsOverride = true
	if override.OriginalSchedule != nil {
		info.YAMLConfig = override.OriginalSchedule
		return
	}
	info.YAMLConfig = buildScheduleFromOverride(override.Values)
}

func buildScheduleFromOverride(values map[string]any) *workflow.Schedule {
	scheduleConfig := &workflow.Schedule{}
	if values == nil {
		return scheduleConfig
	}
	if cronVal, ok := values["original_cron"].(string); ok {
		scheduleConfig.Cron = cronVal
	}
	if enabledVal, ok := values["original_enabled"].(bool); ok {
		scheduleConfig.Enabled = &enabledVal
	}
	if timezoneVal, ok := values["original_timezone"].(string); ok {
		scheduleConfig.Timezone = timezoneVal
	}
	if jitterVal, ok := values["original_jitter"].(string); ok {
		scheduleConfig.Jitter = jitterVal
	}
	if overlapPolicyVal, ok := values["original_overlap_policy"].(string); ok {
		scheduleConfig.OverlapPolicy = workflow.OverlapPolicy(overlapPolicyVal)
	}
	if startAtVal, ok := values["original_start_at"].(time.Time); ok {
		scheduleConfig.StartAt = &startAtVal
	}
	if endAtVal, ok := values["original_end_at"].(time.Time); ok {
		scheduleConfig.EndAt = &endAtVal
	}
	if inputVal, ok := values["original_input"].(map[string]any); ok {
		scheduleConfig.Input = inputVal
	}
	return scheduleConfig
}

func populateScheduleRunTimes(info *Info, desc *client.ScheduleDescription) {
	if len(desc.Info.NextActionTimes) > 0 {
		info.NextRunTime = desc.Info.NextActionTimes[0]
	}
	if len(desc.Info.RecentActions) > 0 {
		lastAction := desc.Info.RecentActions[0]
		info.LastRunTime = &lastAction.ScheduleTime
		info.LastRunStatus = "unknown"
	}
}

// ValidateCronExpression validates a cron expression with optional logging context
// Compozy supports two formats:
//  1. @every syntax for intervals: "@every 15s", "@every 1h30m"
//  2. 6-field cron expressions with seconds:
//     Format: "second minute hour day-of-month month day-of-week"
//     Example: "0 0 9 * * 1-5" (Every weekday at 9:00:00 AM)
//     Note: When sending to Temporal, we automatically append year field
func ValidateCronExpression(ctx context.Context, cronExpr string, workflowID string) error {
	handled, err := validateEveryExpression(ctx, cronExpr, workflowID)
	if handled {
		return err
	}
	return validateCronByFieldCount(ctx, normalizeCronMacros(cronExpr), workflowID)
}

// validateCronByFieldCount routes cron validation based on field count.
// It keeps the switching logic separate so the public validator stays small.
func validateCronByFieldCount(ctx context.Context, cronExpr string, workflowID string) error {
	fieldCount := len(strings.Fields(cronExpr))
	switch fieldCount {
	case 6:
		return validateSixFieldCron(ctx, cronExpr, workflowID)
	case 7:
		return validateSevenFieldCron(ctx, cronExpr, workflowID)
	default:
		return reportInvalidCronFieldCount(ctx, cronExpr, workflowID, fieldCount)
	}
}

func validateEveryExpression(ctx context.Context, cronExpr string, workflowID string) (bool, error) {
	if durationStr, ok := strings.CutPrefix(cronExpr, "@every "); ok {
		if _, err := time.ParseDuration(durationStr); err != nil {
			logger.FromContext(ctx).Error(
				"Schedule validation failed - invalid @every duration",
				"workflow_id", workflowID,
				"cron", cronExpr,
				"error", err,
			)
			return true, fmt.Errorf("invalid @every duration '%s': %w", durationStr, err)
		}
		return true, nil
	}
	return false, nil
}

func normalizeCronMacros(cronExpr string) string {
	switch cronExpr {
	case "@yearly", "@annually":
		return "0 0 0 1 1 *"
	case "@monthly":
		return "0 0 0 1 * *"
	case "@weekly":
		return "0 0 0 * * 0"
	case "@daily", "@midnight":
		return "0 0 0 * * *"
	case "@hourly":
		return "0 0 * * * *"
	default:
		return cronExpr
	}
}

func validateSixFieldCron(ctx context.Context, cronExpr string, workflowID string) error {
	if _, err := cronParser6Fields.Parse(cronExpr); err != nil {
		logger.FromContext(ctx).Error(
			"Schedule validation failed",
			"workflow_id", workflowID,
			"error", err,
			"cron", cronExpr,
		)
		return fmt.Errorf("invalid 6-field cron expression '%s': %w", cronExpr, err)
	}
	return nil
}

func validateSevenFieldCron(ctx context.Context, cronExpr string, workflowID string) error {
	fields := strings.Fields(cronExpr)
	sixFields := strings.Join(fields[:6], " ")
	if _, err := cronParser6Fields.Parse(sixFields); err != nil {
		logger.FromContext(ctx).Error(
			"Schedule validation failed",
			"workflow_id", workflowID,
			"error", err,
			"cron", cronExpr,
		)
		return fmt.Errorf("invalid 7-field cron expression '%s': %w", cronExpr, err)
	}
	yearField := fields[6]
	if isValidYearField(yearField) {
		return nil
	}
	logger.FromContext(ctx).Error(
		"Schedule validation failed - invalid year field",
		"workflow_id", workflowID,
		"cron", cronExpr,
		"year_field", yearField,
	)
	return fmt.Errorf("invalid year field in cron expression '%s': %s", cronExpr, yearField)
}

func reportInvalidCronFieldCount(
	ctx context.Context,
	cronExpr string,
	workflowID string,
	fieldCount int,
) error {
	logger.FromContext(ctx).Error(
		"Schedule validation failed - incorrect field count",
		"workflow_id", workflowID,
		"cron", cronExpr,
		"expected_fields", "6 or 7",
		"actual_fields", fieldCount,
	)
	return fmt.Errorf(
		"invalid cron expression '%s': expected 6 or 7 fields "+
			"(second minute hour day month weekday [year]), got %d fields",
		cronExpr,
		fieldCount,
	)
}

// validateCronForSchedule validates cron expression for schedule creation
func (m *manager) validateCronForSchedule(ctx context.Context, wf *workflow.Config) error {
	return ValidateCronExpression(ctx, wf.Schedule.Cron, wf.ID)
}

// createSchedule creates a new schedule in Temporal
func (m *manager) createSchedule(ctx context.Context, scheduleID string, wf *workflow.Config) error {
	return m.runScheduleOperation(ctx, OperationCreate, func(status *string) error {
		if err := m.validateCronForSchedule(ctx, wf); err != nil {
			return err
		}
		spec, err := m.buildScheduleSpec(ctx, scheduleID, wf)
		if err != nil {
			return err
		}
		action := m.buildScheduleAction(wf)
		options := m.buildScheduleOptions(scheduleID, &spec, action, wf)
		m.warnUnsupportedOverlapPolicy(ctx, scheduleID, wf)
		handle, err := m.client.ScheduleClient().Create(ctx, options)
		if err != nil {
			return fmt.Errorf("failed to create schedule: %w", err)
		}
		*status = OperationStatusSuccess
		logger.FromContext(ctx).
			With("schedule_id", scheduleID, "workflow_id", wf.ID).
			Info("Schedule created successfully", "handle", handle.GetID())
		return nil
	})
}

// updateSchedule updates an existing schedule in Temporal
func (m *manager) updateSchedule(ctx context.Context, scheduleID string, wf *workflow.Config) error {
	return m.runScheduleOperation(ctx, OperationUpdate, func(status *string) error {
		if err := m.validateCronForSchedule(ctx, wf); err != nil {
			return err
		}
		handle := m.client.ScheduleClient().GetHandle(ctx, scheduleID)
		desc, err := handle.Describe(ctx)
		if err != nil {
			return fmt.Errorf("failed to describe schedule for update: %w", err)
		}
		needsUpdate, expectedTimezone, expectedEnabled := m.checkScheduleNeedsUpdate(desc, wf)
		log := logger.FromContext(ctx).With("schedule_id", scheduleID, "workflow_id", wf.ID)
		if !needsUpdate {
			*status = OperationStatusSuccess
			log.Debug("Schedule is up to date, skipping update")
			return nil
		}
		updateFn := m.buildScheduleUpdater(ctx, scheduleID, wf, expectedTimezone, expectedEnabled)
		if err := handle.Update(ctx, client.ScheduleUpdateOptions{DoUpdate: updateFn}); err != nil {
			return fmt.Errorf("failed to update schedule: %w", err)
		}
		*status = OperationStatusSuccess
		log.Info("Schedule updated successfully")
		return nil
	})
}

func (m *manager) deferOperationMetric(ctx context.Context, operation string, status *string) func() {
	if m.metrics == nil {
		return func() {}
	}
	return func() {
		if status == nil {
			return
		}
		m.metrics.RecordOperation(ctx, operation, *status, m.projectID)
	}
}

// runScheduleOperation wraps schedule mutations with metric tracking.
// It lets the caller mark success on the provided status pointer.
func (m *manager) runScheduleOperation(
	ctx context.Context,
	operation string,
	fn func(status *string) error,
) error {
	status := OperationStatusFailure
	defer m.deferOperationMetric(ctx, operation, &status)()
	if err := fn(&status); err != nil {
		return err
	}
	return nil
}

func (m *manager) buildScheduleSpec(
	ctx context.Context,
	scheduleID string,
	wf *workflow.Config,
) (client.ScheduleSpec, error) {
	spec := client.ScheduleSpec{
		CronExpressions: []string{EnsureTemporalCron(wf.Schedule.Cron)},
		TimeZoneName:    m.scheduleTimezone(wf),
	}
	if jitter, hasJitter, err := m.parseScheduleJitter(ctx, scheduleID, wf); err != nil {
		return client.ScheduleSpec{}, err
	} else if hasJitter {
		spec.Jitter = jitter
	}
	m.applyScheduleWindow(wf, &spec)
	return spec, nil
}

func (m *manager) parseScheduleJitter(
	ctx context.Context,
	scheduleID string,
	wf *workflow.Config,
) (time.Duration, bool, error) {
	jitterValue := wf.Schedule.Jitter
	if jitterValue == "" {
		return 0, false, nil
	}
	duration, err := time.ParseDuration(jitterValue)
	if err != nil {
		logger.FromContext(ctx).
			With("schedule_id", scheduleID, "workflow_id", wf.ID).
			Error("Schedule validation failed", "error", err, "jitter", jitterValue)
		return 0, false, fmt.Errorf("invalid jitter duration: %w", err)
	}
	return duration, true, nil
}

func (m *manager) scheduleTimezone(wf *workflow.Config) string {
	if wf.Schedule.Timezone != "" {
		return wf.Schedule.Timezone
	}
	return DefaultTimezone
}

func (m *manager) applyScheduleWindow(wf *workflow.Config, spec *client.ScheduleSpec) {
	if wf.Schedule.StartAt != nil {
		spec.StartAt = *wf.Schedule.StartAt
	} else {
		spec.StartAt = time.Time{}
	}
	if wf.Schedule.EndAt != nil {
		spec.EndAt = *wf.Schedule.EndAt
	} else {
		spec.EndAt = time.Time{}
	}
}

func (m *manager) buildScheduleAction(wf *workflow.Config) *client.ScheduleWorkflowAction {
	action := &client.ScheduleWorkflowAction{
		ID:                       wf.ID,
		Workflow:                 worker.CompozyWorkflowName,
		TaskQueue:                m.taskQueue,
		WorkflowExecutionTimeout: 0,
		WorkflowRunTimeout:       0,
		WorkflowTaskTimeout:      0,
	}
	m.populateScheduleAction(action, wf)
	return action
}

func (m *manager) populateScheduleAction(action *client.ScheduleWorkflowAction, wf *workflow.Config) {
	action.Workflow = worker.CompozyWorkflowName
	action.Args = []any{triggerInputForWorkflow(wf)}
}

func triggerInputForWorkflow(wf *workflow.Config) map[string]any {
	return map[string]any{
		"workflow_id":      wf.ID,
		"workflow_exec_id": "",
		"input":            wf.Schedule.Input,
	}
}

func (m *manager) buildScheduleOptions(
	scheduleID string,
	spec *client.ScheduleSpec,
	action *client.ScheduleWorkflowAction,
	wf *workflow.Config,
) client.ScheduleOptions {
	paused := wf.Schedule.Enabled != nil && !*wf.Schedule.Enabled
	return client.ScheduleOptions{
		ID:     scheduleID,
		Spec:   *spec,
		Action: action,
		Paused: paused,
		Memo: map[string]any{
			"project_id":  m.projectID,
			"workflow_id": wf.ID,
		},
	}
}

func (m *manager) warnUnsupportedOverlapPolicy(ctx context.Context, scheduleID string, wf *workflow.Config) {
	policy := wf.Schedule.OverlapPolicy
	if policy == "" || policy == workflow.OverlapSkip {
		return
	}
	logger.FromContext(ctx).
		With("schedule_id", scheduleID, "workflow_id", wf.ID).
		Warn(
			"OverlapPolicy is configured but not enforced by schedule creation in this SDK version",
			"policy", policy,
			"info", "Policy must be handled by the workflow or custom trigger logic",
		)
}

func (m *manager) buildScheduleUpdater(
	ctx context.Context,
	scheduleID string,
	wf *workflow.Config,
	expectedTimezone string,
	expectedEnabled bool,
) func(client.ScheduleUpdateInput) (*client.ScheduleUpdate, error) {
	return func(input client.ScheduleUpdateInput) (*client.ScheduleUpdate, error) {
		spec := input.Description.Schedule.Spec
		if spec == nil {
			spec = &client.ScheduleSpec{}
			input.Description.Schedule.Spec = spec
		}
		spec.CronExpressions = []string{EnsureTemporalCron(wf.Schedule.Cron)}
		spec.TimeZoneName = expectedTimezone
		if jitter, hasJitter, err := m.parseScheduleJitter(ctx, scheduleID, wf); err != nil {
			return nil, err
		} else if hasJitter {
			spec.Jitter = jitter
		} else {
			spec.Jitter = 0
		}
		m.applyScheduleWindow(wf, spec)
		input.Description.Schedule.State.Paused = !expectedEnabled
		if action, ok := input.Description.Schedule.Action.(*client.ScheduleWorkflowAction); ok {
			m.populateScheduleAction(action, wf)
		}
		return &client.ScheduleUpdate{Schedule: &input.Description.Schedule}, nil
	}
}

// deleteSchedule deletes a schedule from Temporal
func (m *manager) deleteSchedule(ctx context.Context, scheduleID string) error {
	log := logger.FromContext(ctx).With("schedule_id", scheduleID)
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
	needsUpdate = m.checkCronNeedsUpdate(currentSpec, wf) ||
		m.checkTimezoneNeedsUpdate(currentSpec, wf) ||
		m.checkEnabledStateNeedsUpdate(desc, wf) ||
		m.checkJitterNeedsUpdate(currentSpec, wf) ||
		m.checkStartEndTimesNeedsUpdate(currentSpec, wf)
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
	if (wf.Schedule.StartAt == nil && !currentSpec.StartAt.IsZero()) ||
		(wf.Schedule.StartAt != nil && !currentSpec.StartAt.Equal(*wf.Schedule.StartAt)) {
		return true
	}
	if (wf.Schedule.EndAt == nil && !currentSpec.EndAt.IsZero()) ||
		(wf.Schedule.EndAt != nil && !currentSpec.EndAt.Equal(*wf.Schedule.EndAt)) {
		return true
	}
	return false
}
