package schedule

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gosimple/slug"
	"github.com/sethvargo/go-retry"
	"go.opentelemetry.io/otel/metric"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

// Manager handles the lifecycle of scheduled workflows in Temporal
type Manager interface {
	// ReconcileSchedules performs stateless reconciliation between workflows and Temporal schedules
	ReconcileSchedules(ctx context.Context, workflows []*workflow.Config) error
	// ListSchedules returns all scheduled workflows with their current status
	ListSchedules(ctx context.Context) ([]*Info, error)
	// GetSchedule returns details of a specific scheduled workflow
	GetSchedule(ctx context.Context, workflowID string) (*Info, error)
	// UpdateSchedule updates a schedule (for temporary overrides)
	UpdateSchedule(ctx context.Context, workflowID string, update UpdateRequest) error
	// DeleteSchedule removes a schedule from Temporal
	DeleteSchedule(ctx context.Context, workflowID string) error
	// OnConfigurationReload handles workflow configuration reload events
	OnConfigurationReload(ctx context.Context, workflows []*workflow.Config) error
	// StartPeriodicReconciliation starts a background goroutine for periodic reconciliation
	StartPeriodicReconciliation(
		ctx context.Context,
		getWorkflows func() []*workflow.Config,
		interval time.Duration,
	) error
	// StopPeriodicReconciliation stops the periodic reconciliation goroutine
	StopPeriodicReconciliation()
}

// Info contains information about a scheduled workflow
type Info struct {
	WorkflowID    string             `json:"workflow_id"`
	ScheduleID    string             `json:"schedule_id"`
	Cron          string             `json:"cron"`
	Timezone      string             `json:"timezone"`
	Enabled       bool               `json:"enabled"`
	IsOverride    bool               `json:"is_override"` // API modification
	YAMLConfig    *workflow.Schedule `json:"yaml_config,omitempty"`
	NextRunTime   time.Time          `json:"next_run_time"`
	LastRunTime   *time.Time         `json:"last_run_time,omitempty"`
	LastRunStatus string             `json:"last_run_status,omitempty"`
}

// UpdateRequest contains fields that can be updated via API
type UpdateRequest struct {
	Enabled *bool   `json:"enabled"`
	Cron    *string `json:"cron"`
}

// Override represents a persistent API override with timestamp tracking
type Override struct {
	WorkflowID       string             `json:"workflow_id"`
	ModifiedAt       time.Time          `json:"modified_at"`
	Values           map[string]any     `json:"values"`
	OriginalSchedule *workflow.Schedule `json:"original_schedule,omitempty"`
}

// OverrideCache manages persistent API overrides with thread-safe access
type OverrideCache struct {
	mu        sync.RWMutex
	overrides map[string]*Override
}

// NewOverrideCache creates a new override cache
func NewOverrideCache() *OverrideCache {
	return &OverrideCache{
		overrides: make(map[string]*Override),
	}
}

// Config holds configuration options for the schedule manager
type Config struct {
	// PageSize for listing schedules from Temporal (default: 100)
	PageSize int
}

// DefaultConfig returns default configuration values
func DefaultConfig() *Config {
	return &Config{
		PageSize: 100,
	}
}

// manager implements the Manager interface
type manager struct {
	client    *worker.Client
	projectID string
	taskQueue string
	config    *Config
	mu        sync.RWMutex
	// Track API overrides with persistence and timestamp tracking
	overrideCache *OverrideCache
	// Cache for last-known YAML modification times to preserve overrides on filesystem errors
	lastKnownModTimes map[string]time.Time
	// Metrics for observability
	metrics            *Metrics
	lastScheduleCounts map[string]int
	// Periodic reconciliation support
	periodicCancel context.CancelFunc
	periodicWG     sync.WaitGroup
	// Reconciliation mutex to prevent concurrent reconciliations
	reconcileMu sync.Mutex
}

// ShouldSkipReconciliation checks if a workflow should be skipped due to recent API overrides
func (c *OverrideCache) ShouldSkipReconciliation(workflowID string, yamlModTime time.Time) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	override, exists := c.overrides[workflowID]
	if !exists {
		return false
	}
	// Skip if override is newer than YAML
	return override.ModifiedAt.After(yamlModTime)
}

// SetOverride stores an API override for a workflow
func (c *OverrideCache) SetOverride(workflowID string, values map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.overrides[workflowID] = &Override{
		WorkflowID: workflowID,
		ModifiedAt: time.Now(),
		Values:     values,
	}
}

// SetOverrideWithSchedule stores an API override with the original schedule config
func (c *OverrideCache) SetOverrideWithSchedule(
	workflowID string,
	values map[string]any,
	originalSchedule *workflow.Schedule,
) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.overrides[workflowID] = &Override{
		WorkflowID:       workflowID,
		ModifiedAt:       time.Now(),
		Values:           values,
		OriginalSchedule: originalSchedule,
	}
}

// GetOverride retrieves an override for a workflow
func (c *OverrideCache) GetOverride(workflowID string) (*Override, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	override, exists := c.overrides[workflowID]
	if !exists {
		return nil, false
	}
	// Return a copy to prevent concurrent modification
	result := &Override{
		WorkflowID: override.WorkflowID,
		ModifiedAt: override.ModifiedAt,
		Values:     copyValues(override.Values),
	}
	// Deep copy the original schedule if present
	if override.OriginalSchedule != nil {
		copiedSchedule, err := core.DeepCopy(override.OriginalSchedule)
		if err == nil {
			result.OriginalSchedule = copiedSchedule
		}
		// If error, continue with nil OriginalSchedule
	}
	return result, true
}

// ClearOverride removes an override for a workflow
func (c *OverrideCache) ClearOverride(workflowID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, existed := c.overrides[workflowID]
	delete(c.overrides, workflowID)
	return existed
}

// ListOverrides returns all current overrides
func (c *OverrideCache) ListOverrides() map[string]*Override {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string]*Override, len(c.overrides))
	for k, v := range c.overrides {
		override := &Override{
			WorkflowID: v.WorkflowID,
			ModifiedAt: v.ModifiedAt,
			Values:     copyValues(v.Values),
		}
		// Deep copy the original schedule if present
		if v.OriginalSchedule != nil {
			copiedSchedule, err := core.DeepCopy(v.OriginalSchedule)
			if err == nil {
				override.OriginalSchedule = copiedSchedule
			}
		}
		result[k] = override
	}
	return result
}

// copyValues creates a deep copy of the values map to prevent concurrent modification.
// Uses the existing core.DeepCopy method for reliable deep copying of all types.
func copyValues(original map[string]any) map[string]any {
	if original == nil {
		return nil
	}
	copied, err := core.DeepCopy(original)
	if err != nil {
		// Since this is a package-level function without logger access,
		// we cannot log the error. The calling code should handle errors appropriately.
		// Fallback to empty map on copy failure to prevent panics
		// This maintains the function's contract of always returning a valid map
		return make(map[string]any)
	}
	return copied
}

// getYAMLModTime gets the modification time of a workflow's YAML file
// Returns zero time on errors to preserve overrides during transient filesystem issues
func (m *manager) getYAMLModTime(ctx context.Context, wf *workflow.Config) time.Time {
	filePath := wf.GetFilePath()
	if filePath == "" {
		return time.Time{} // Return zero time to preserve overrides
	}
	stat, err := os.Stat(filePath)
	if err != nil {
		log := logger.FromContext(ctx)
		m.mu.RLock()
		lastKnownTime, hasLastKnown := m.lastKnownModTimes[wf.ID]
		m.mu.RUnlock()
		if hasLastKnown {
			log.Warn(
				"Failed to get file modification time, using last known time to preserve overrides",
				"workflow_id", wf.ID,
				"path", filePath,
				"last_known_time", lastKnownTime,
				"error", err,
			)
			return lastKnownTime
		}
		log.Warn(
			"Failed to get file modification time and no cached time available, returning zero time to preserve overrides",
			"workflow_id",
			wf.ID,
			"path",
			filePath,
			"error",
			err,
		)
		return time.Time{} // Return zero time to preserve overrides
	}
	modTime := stat.ModTime()
	// Cache the successful modification time
	m.mu.Lock()
	m.lastKnownModTimes[wf.ID] = modTime
	m.mu.Unlock()
	return modTime
}

// NewManager creates a new schedule manager
func NewManager(client *worker.Client, projectID string) Manager {
	return NewManagerWithConfig(client, projectID, DefaultConfig())
}

// NewManagerWithConfig creates a new schedule manager with custom configuration
func NewManagerWithConfig(client *worker.Client, projectID string, config *Config) Manager {
	if config == nil {
		config = DefaultConfig()
	}
	return &manager{
		client:             client,
		projectID:          projectID,
		taskQueue:          slugify(projectID),
		config:             config,
		overrideCache:      NewOverrideCache(),
		lastKnownModTimes:  make(map[string]time.Time),
		metrics:            nil, // Will be set by SetMetrics if monitoring is enabled
		lastScheduleCounts: make(map[string]int),
	}
}

// NewManagerWithMetrics creates a new schedule manager with metrics
func NewManagerWithMetrics(ctx context.Context, client *worker.Client, projectID string, meter metric.Meter) Manager {
	return NewManagerWithMetricsAndConfig(ctx, client, projectID, meter, DefaultConfig())
}

// NewManagerWithMetricsAndConfig creates a new schedule manager with metrics and custom configuration
func NewManagerWithMetricsAndConfig(
	ctx context.Context,
	client *worker.Client,
	projectID string,
	meter metric.Meter,
	config *Config,
) Manager {
	if config == nil {
		config = DefaultConfig()
	}
	return &manager{
		client:             client,
		projectID:          projectID,
		taskQueue:          slugify(projectID),
		config:             config,
		overrideCache:      NewOverrideCache(),
		lastKnownModTimes:  make(map[string]time.Time),
		metrics:            NewMetrics(ctx, meter),
		lastScheduleCounts: make(map[string]int),
	}
}

// ReconcileSchedules performs stateless reconciliation between workflows and Temporal schedules
func (m *manager) ReconcileSchedules(ctx context.Context, workflows []*workflow.Config) error {
	m.reconcileMu.Lock()
	defer m.reconcileMu.Unlock()
	log := logger.FromContext(ctx).With("project", m.projectID)
	log.Info("Starting schedule reconciliation", "workflow_count", len(workflows))

	// Track reconciliation with metrics
	var tracker *ReconciliationTracker
	if m.metrics != nil {
		tracker = m.metrics.NewReconciliationTracker(ctx, m.projectID)
		defer tracker.Finish()
	}

	startTime := time.Now()
	// 1. Get existing schedules and build desired state
	existingSchedules, desiredSchedules, yamlModTimes, err := m.buildReconciliationState(ctx, workflows)
	if err != nil {
		log.Error(
			"Failed to list existing schedules from Temporal, proceeding with partial reconciliation",
			"error",
			err,
		)
		// This is the resilience: we proceed with an empty map for existing schedules
		existingSchedules = make(map[string]client.ScheduleHandle)
	}
	// If desiredSchedules is nil (which shouldn't happen with the refactor, but is good defense), we cannot proceed
	if desiredSchedules == nil {
		return fmt.Errorf("cannot proceed with reconciliation without a desired state")
	}

	// 2. Determine operations needed (respecting active overrides)
	toCreate, toUpdate, toDelete, _ := m.planReconciliationOperations(
		ctx, existingSchedules, desiredSchedules, yamlModTimes)

	// 3. Execute reconciliation
	if err := m.executeReconciliation(ctx, toCreate, toUpdate, toDelete); err != nil {
		return fmt.Errorf("reconciliation failed: %w", err)
	}

	// 4. Update metrics and log completion
	m.finishReconciliation(ctx, desiredSchedules, toCreate, toUpdate, toDelete, startTime)
	return nil
}

// buildReconciliationState gets existing schedules and builds desired state maps
// Returns partial results when possible to enable resilient reconciliation
func (m *manager) buildReconciliationState(ctx context.Context, workflows []*workflow.Config) (
	map[string]client.ScheduleHandle, map[string]*workflow.Config, map[string]time.Time, error) {
	log := logger.FromContext(ctx)

	// Build desired state from YAML first
	desiredSchedules := make(map[string]*workflow.Config)
	yamlModTimes := make(map[string]time.Time)
	for _, wf := range workflows {
		if wf.Schedule != nil {
			scheduleID := m.scheduleID(wf.ID)
			desiredSchedules[scheduleID] = wf
			yamlModTimes[wf.ID] = m.getYAMLModTime(ctx, wf)
		}
	}
	log.Debug("Built desired state", "count", len(desiredSchedules))

	// Get all existing schedules from Temporal
	existingSchedules, err := m.listSchedulesByPrefix(ctx, m.schedulePrefix())
	if err != nil {
		// Return error along with successfully built desired state
		return nil, desiredSchedules, yamlModTimes, fmt.Errorf("failed to list existing schedules: %w", err)
	}
	log.Debug("Found existing schedules", "count", len(existingSchedules))

	return existingSchedules, desiredSchedules, yamlModTimes, nil
}

// planReconciliationOperations determines which operations are needed
func (m *manager) planReconciliationOperations(
	ctx context.Context,
	existingSchedules map[string]client.ScheduleHandle,
	desiredSchedules map[string]*workflow.Config,
	yamlModTimes map[string]time.Time,
) (map[string]*workflow.Config, map[string]*workflow.Config, []string, []string) {
	log := logger.FromContext(ctx)
	toCreate := make(map[string]*workflow.Config)
	toUpdate := make(map[string]*workflow.Config)
	toDelete := make([]string, 0)
	skippedDueToOverrides := make([]string, 0)

	// Find schedules to create or update
	for scheduleID, wf := range desiredSchedules {
		workflowID := m.workflowIDFromScheduleID(scheduleID)
		yamlModTime := yamlModTimes[workflowID]
		// Check if this workflow should be skipped due to recent API overrides
		if m.overrideCache.ShouldSkipReconciliation(workflowID, yamlModTime) {
			skippedDueToOverrides = append(skippedDueToOverrides, workflowID)
			log.Debug("Skipping reconciliation due to active API override", "workflow_id", workflowID)
			continue
		}
		// If we are not skipping, the YAML is the source of truth. Clear any stale override.
		if m.overrideCache.ClearOverride(workflowID) {
			log.Info("Cleared stale API override due to newer YAML configuration", "workflow_id", workflowID)
		}
		if _, exists := existingSchedules[scheduleID]; exists {
			toUpdate[scheduleID] = wf
		} else {
			toCreate[scheduleID] = wf
		}
	}

	// Find schedules to delete
	for scheduleID := range existingSchedules {
		if _, desired := desiredSchedules[scheduleID]; !desired {
			toDelete = append(toDelete, scheduleID)
		}
	}

	log.Info("Reconciliation plan",
		"to_create", len(toCreate),
		"to_update", len(toUpdate),
		"to_delete", len(toDelete),
		"skipped_overrides", len(skippedDueToOverrides),
	)
	if len(skippedDueToOverrides) > 0 {
		log.Debug("Skipped workflows due to API overrides", "workflows", skippedDueToOverrides)
	}

	return toCreate, toUpdate, toDelete, skippedDueToOverrides
}

// finishReconciliation updates metrics and logs completion
func (m *manager) finishReconciliation(
	ctx context.Context,
	desiredSchedules map[string]*workflow.Config,
	toCreate, toUpdate map[string]*workflow.Config,
	toDelete []string,
	startTime time.Time,
) {
	log := logger.FromContext(ctx)

	// Update workflow count metrics
	if m.metrics != nil {
		m.updateWorkflowCountMetrics(ctx, desiredSchedules)
	}

	duration := time.Since(startTime)
	log.Info("Schedule reconciliation completed",
		"duration", duration,
		"created", len(toCreate),
		"updated", len(toUpdate),
		"deleted", len(toDelete),
	)
}

// updateWorkflowCountMetrics calculates and reports workflow count deltas
func (m *manager) updateWorkflowCountMetrics(ctx context.Context, desiredSchedules map[string]*workflow.Config) {
	currentCounts := map[string]int{
		"active":   0,
		"paused":   0,
		"override": 0,
	}
	// Calculate current counts based on desired state and overrides
	for scheduleID, wf := range desiredSchedules {
		workflowID := m.workflowIDFromScheduleID(scheduleID)
		if _, hasOverride := m.overrideCache.GetOverride(workflowID); hasOverride {
			currentCounts["override"]++
		} else if wf.Schedule.Enabled != nil && !*wf.Schedule.Enabled {
			currentCounts["paused"]++
		} else {
			currentCounts["active"]++
		}
	}

	// Minimize mutex holding time by computing deltas first
	var deltas []struct {
		status string
		delta  int64
	}

	// Only lock to read/update lastScheduleCounts
	m.mu.Lock()
	// Check if this is the first run (no previous counts recorded)
	isFirstRun := len(m.lastScheduleCounts) == 0

	// Calculate deltas - combine keys from both maps
	allStatuses := make(map[string]struct{})
	for status := range currentCounts {
		allStatuses[status] = struct{}{}
	}
	for status := range m.lastScheduleCounts {
		allStatuses[status] = struct{}{}
	}

	for status := range allStatuses {
		newCount := int64(currentCounts[status])
		lastCount := int64(m.lastScheduleCounts[status])
		delta := newCount - lastCount
		// On first run, report absolute values even if they're 0 to establish baseline
		// On subsequent runs, only report non-zero deltas
		if isFirstRun || delta != 0 {
			deltas = append(deltas, struct {
				status string
				delta  int64
			}{status: status, delta: delta})
		}
	}
	m.lastScheduleCounts = currentCounts
	m.mu.Unlock()

	// Report metrics outside the mutex lock to avoid blocking on I/O
	for _, d := range deltas {
		m.metrics.UpdateWorkflowCount(ctx, m.projectID, d.status, d.delta)
	}
}

// ListSchedules returns all scheduled workflows
func (m *manager) ListSchedules(ctx context.Context) ([]*Info, error) {
	log := logger.FromContext(ctx)
	schedules, err := m.listSchedulesByPrefix(ctx, m.schedulePrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to list schedules: %w", err)
	}
	result := make([]*Info, 0, len(schedules))
	for scheduleID, handle := range schedules {
		info, err := m.getScheduleInfo(ctx, scheduleID, handle)
		if err != nil {
			log.Warn("Failed to get schedule info", "schedule_id", scheduleID, "error", err)
			continue
		}
		result = append(result, info)
	}
	return result, nil
}

// GetSchedule returns details of a specific scheduled workflow
func (m *manager) GetSchedule(ctx context.Context, workflowID string) (*Info, error) {
	scheduleID := m.scheduleID(workflowID)
	handle := m.client.ScheduleClient().GetHandle(ctx, scheduleID)
	info, err := m.getScheduleInfo(ctx, scheduleID, handle)
	if err != nil {
		// Check if this is a "not found" error and return more specific message
		var notFoundErr *serviceerror.NotFound
		if errors.As(err, &notFoundErr) {
			return nil, ErrScheduleNotFound
		}
		return nil, fmt.Errorf("failed to get schedule for workflow %s: %w", workflowID, err)
	}
	return info, nil
}

// UpdateSchedule updates a schedule (for temporary overrides)
func (m *manager) UpdateSchedule(ctx context.Context, workflowID string, update UpdateRequest) error {
	scheduleID := m.scheduleID(workflowID)
	handle := m.client.ScheduleClient().GetHandle(ctx, scheduleID)

	// Get current schedule description
	desc, err := m.getScheduleDescription(ctx, handle)
	if err != nil {
		return err
	}

	// Log the API override operation
	m.logAPIOverrideOperation(ctx, update)

	// Prepare override values
	values, err := m.prepareOverrideValues(ctx, workflowID, desc, update)
	if err != nil {
		return err
	}

	// Construct original schedule from current Temporal state
	originalSchedule := m.constructScheduleFromDescription(desc)

	// Store the override in cache with original schedule
	m.overrideCache.SetOverrideWithSchedule(workflowID, values, originalSchedule)

	// Update in Temporal
	if err := m.updateScheduleInTemporal(ctx, handle, update); err != nil {
		// Remove override on failure
		m.overrideCache.ClearOverride(workflowID)
		return fmt.Errorf("failed to update schedule %s: %w", workflowID, err)
	}
	return nil
}

// getScheduleDescription gets the schedule description and handles errors
func (m *manager) getScheduleDescription(
	ctx context.Context,
	handle client.ScheduleHandle,
) (*client.ScheduleDescription, error) {
	desc, err := handle.Describe(ctx)
	if err != nil {
		// Check if this is a "not found" error
		var notFoundErr *serviceerror.NotFound
		if errors.As(err, &notFoundErr) {
			return nil, ErrScheduleNotFound
		}
		return nil, fmt.Errorf("failed to describe schedule before update: %w", err)
	}
	return desc, nil
}

// logAPIOverrideOperation logs the API override operation with appropriate actions
func (m *manager) logAPIOverrideOperation(ctx context.Context, update UpdateRequest) {
	log := logger.FromContext(ctx)
	var actions []string
	if update.Enabled != nil {
		if *update.Enabled {
			actions = append(actions, "enable")
		} else {
			actions = append(actions, "disable")
		}
	}
	if update.Cron != nil {
		actions = append(actions, "update_cron")
	}

	action := "unknown"
	if len(actions) > 0 {
		action = strings.Join(actions, ", ")
	}

	log.Warn("Schedule modified via API",
		"action", action,
		"will_revert_on_reload", true)
}

// constructScheduleFromDescription creates a workflow.Schedule from Temporal description
func (m *manager) constructScheduleFromDescription(desc *client.ScheduleDescription) *workflow.Schedule {
	schedule := &workflow.Schedule{}

	// Extract cron expression
	if len(desc.Schedule.Spec.CronExpressions) > 0 {
		schedule.Cron = desc.Schedule.Spec.CronExpressions[0]
	}

	// Extract enabled state
	enabled := !desc.Schedule.State.Paused
	schedule.Enabled = &enabled

	// Extract timezone
	schedule.Timezone = desc.Schedule.Spec.TimeZoneName
	if schedule.Timezone == "" {
		schedule.Timezone = DefaultTimezone
	}

	// Extract jitter
	if desc.Schedule.Spec.Jitter > 0 {
		schedule.Jitter = desc.Schedule.Spec.Jitter.String()
	}

	// Extract start/end times
	if !desc.Schedule.Spec.StartAt.IsZero() {
		startAt := desc.Schedule.Spec.StartAt
		schedule.StartAt = &startAt
	}
	if !desc.Schedule.Spec.EndAt.IsZero() {
		endAt := desc.Schedule.Spec.EndAt
		schedule.EndAt = &endAt
	}

	// Extract workflow input from action
	if action, ok := desc.Schedule.Action.(*client.ScheduleWorkflowAction); ok && len(action.Args) > 0 {
		if triggerInput, ok := action.Args[0].(map[string]any); ok {
			if input, ok := triggerInput["input"].(map[string]any); ok {
				schedule.Input = input
			}
		}
	}

	return schedule
}

// prepareOverrideValues prepares the override values from current state and update request
func (m *manager) prepareOverrideValues(
	ctx context.Context,
	workflowID string,
	desc *client.ScheduleDescription,
	update UpdateRequest,
) (map[string]any, error) {
	values := make(map[string]any)

	// Store original values from current Temporal state
	if len(desc.Schedule.Spec.CronExpressions) > 0 {
		values["original_cron"] = desc.Schedule.Spec.CronExpressions[0]
	}
	values["original_enabled"] = !desc.Schedule.State.Paused
	if desc.Schedule.Spec.TimeZoneName != "" {
		values["original_timezone"] = desc.Schedule.Spec.TimeZoneName
	}

	// Set new override values
	if update.Enabled != nil {
		values["enabled"] = *update.Enabled
	}
	if update.Cron != nil {
		// Validate cron expression before storing
		if err := m.validateCronExpression(ctx, workflowID, *update.Cron); err != nil {
			return nil, err
		}
		values["cron"] = *update.Cron
	}

	return values, nil
}

// validateCronExpression validates a cron expression
func (m *manager) validateCronExpression(ctx context.Context, workflowID, cronExpr string) error {
	return ValidateCronExpression(ctx, cronExpr, workflowID)
}

// updateScheduleInTemporal updates the schedule in Temporal
func (m *manager) updateScheduleInTemporal(
	ctx context.Context,
	handle client.ScheduleHandle,
	update UpdateRequest,
) error {
	return handle.Update(ctx, client.ScheduleUpdateOptions{
		DoUpdate: func(schedule client.ScheduleUpdateInput) (*client.ScheduleUpdate, error) {
			if update.Enabled != nil {
				schedule.Description.Schedule.State.Paused = !*update.Enabled
			}
			if update.Cron != nil {
				// Temporal requires 7 fields, so append year if needed
				cronExpr := EnsureTemporalCron(*update.Cron)
				schedule.Description.Schedule.Spec.CronExpressions = []string{cronExpr}
			}
			return &client.ScheduleUpdate{
				Schedule: &schedule.Description.Schedule,
			}, nil
		},
	})
}

// DeleteSchedule removes a schedule from Temporal
func (m *manager) DeleteSchedule(ctx context.Context, workflowID string) error {
	scheduleID := m.scheduleID(workflowID)
	handle := m.client.ScheduleClient().GetHandle(ctx, scheduleID)
	err := handle.Delete(ctx)
	if err != nil {
		// Check if this is a "not found" error
		var notFoundErr *serviceerror.NotFound
		if errors.As(err, &notFoundErr) {
			return ErrScheduleNotFound
		}
		return fmt.Errorf("failed to delete schedule %s: %w", workflowID, err)
	}
	// Remove any overrides
	m.overrideCache.ClearOverride(workflowID)
	return nil
}

// schedulePrefix returns the prefix for all schedules in this project
func (m *manager) schedulePrefix() string {
	return fmt.Sprintf("schedule-%s-", m.projectID)
}

// scheduleID generates a schedule ID for a workflow
func (m *manager) scheduleID(workflowID string) string {
	return fmt.Sprintf("schedule-%s-%s", m.projectID, workflowID)
}

// workflowIDFromScheduleID extracts the workflow ID from a schedule ID
func (m *manager) workflowIDFromScheduleID(scheduleID string) string {
	prefix := m.schedulePrefix()
	if after, ok := strings.CutPrefix(scheduleID, prefix); ok {
		return after
	}
	return ""
}

// slugify converts a string to a valid Temporal task queue name
// Uses the gosimple/slug library for RFC-compliant URL-friendly slugs
func slugify(s string) string {
	// The slug library handles all edge cases including:
	// - Converting to lowercase
	// - Replacing spaces and special characters with hyphens
	// - Removing non-ASCII characters
	// - Collapsing multiple hyphens
	// - Trimming leading/trailing hyphens
	return slug.Make(s)
}

// isRetryableError determines if an error should be retried
// Returns false for permanent errors like validation errors, authorization errors, etc.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	// Check for specific Temporal service errors that are permanent
	var invalidArgErr *serviceerror.InvalidArgument
	var permissionDeniedErr *serviceerror.PermissionDenied
	var alreadyExistsErr *serviceerror.AlreadyExists
	var notFoundErr *serviceerror.NotFound
	var unimplementedErr *serviceerror.Unimplemented
	// These errors are permanent and should not be retried
	if errors.As(err, &invalidArgErr) ||
		errors.As(err, &permissionDeniedErr) ||
		errors.As(err, &alreadyExistsErr) ||
		errors.As(err, &notFoundErr) ||
		errors.As(err, &unimplementedErr) {
		return false
	}
	// Check for context cancellation/timeout which should not be retried
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	// Default to retryable for network/temporary errors
	// Note: Removed fragile string matching as it could incorrectly classify
	// retryable errors as permanent (e.g., "invalid connection" network errors)
	return true
}

// workItem represents a reconciliation operation
type workItem struct {
	scheduleID string
	wf         *workflow.Config
	operation  string
	execute    func(context.Context) error
}

// executeReconciliation performs the actual reconciliation operations
func (m *manager) executeReconciliation(
	ctx context.Context,
	toCreate, toUpdate map[string]*workflow.Config,
	toDelete []string,
) error {
	// Create work queue with all operations
	totalOps := len(toCreate) + len(toUpdate) + len(toDelete)
	workQueue := make(chan workItem, totalOps)
	errChan := make(chan error, totalOps)

	// Queue all work items
	m.queueCreateOperations(workQueue, toCreate)
	m.queueUpdateOperations(workQueue, toUpdate)
	m.queueDeleteOperations(workQueue, toDelete)
	close(workQueue)

	// Start bounded worker pool
	const maxWorkers = 10
	var wg sync.WaitGroup
	for range maxWorkers {
		wg.Add(1)
		go m.reconciliationWorker(ctx, workQueue, errChan, &wg)
	}

	// Wait for all workers to complete
	wg.Wait()
	close(errChan)

	// Collect any errors
	return m.collectReconciliationErrors(ctx, errChan)
}

// queueCreateOperations queues create operations
func (m *manager) queueCreateOperations(workQueue chan<- workItem, toCreate map[string]*workflow.Config) {
	for scheduleID, wf := range toCreate {
		scheduleID, wf := scheduleID, wf // Capture loop variables
		workQueue <- workItem{
			scheduleID: scheduleID,
			wf:         wf,
			operation:  "create",
			execute: func(ctx context.Context) error {
				if err := m.createSchedule(ctx, scheduleID, wf); err != nil {
					return fmt.Errorf("failed to create schedule %s: %w", scheduleID, err)
				}
				return nil
			},
		}
	}
}

// queueUpdateOperations queues update operations
func (m *manager) queueUpdateOperations(workQueue chan<- workItem, toUpdate map[string]*workflow.Config) {
	for scheduleID, wf := range toUpdate {
		scheduleID, wf := scheduleID, wf // Capture loop variables
		workQueue <- workItem{
			scheduleID: scheduleID,
			wf:         wf,
			operation:  "update",
			execute: func(ctx context.Context) error {
				if err := m.updateSchedule(ctx, scheduleID, wf); err != nil {
					return fmt.Errorf("failed to update schedule %s: %w", scheduleID, err)
				}
				return nil
			},
		}
	}
}

// queueDeleteOperations queues delete operations
func (m *manager) queueDeleteOperations(workQueue chan<- workItem, toDelete []string) {
	for _, scheduleID := range toDelete {
		id := scheduleID // capture
		workQueue <- workItem{
			scheduleID: id,
			operation:  "delete",
			execute: func(ctx context.Context) error {
				if err := m.deleteSchedule(ctx, id); err != nil {
					return fmt.Errorf("failed to delete schedule %s: %w", id, err)
				}
				return nil
			},
		}
	}
}

// reconciliationWorker processes work items from the queue
func (m *manager) reconciliationWorker(
	ctx context.Context,
	workQueue <-chan workItem,
	errChan chan<- error,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	for work := range workQueue {
		if err := m.processWorkItem(ctx, work); err != nil {
			errChan <- err
		}
	}
}

// processWorkItem processes a single work item with retry logic
func (m *manager) processWorkItem(ctx context.Context, work workItem) error {
	log := logger.FromContext(ctx)
	// Check context cancellation
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Execute operation with retry for retryable errors
	err := work.execute(ctx)
	if err != nil && isRetryableError(err) {
		// Retry with backoff, using retry context
		backoff := retry.WithMaxRetries(3, retry.NewExponential(1*time.Second))
		err = retry.Do(ctx, backoff, func(retryCtx context.Context) error {
			// Use the retry context for the operation
			return work.execute(retryCtx)
		})
	}

	if err != nil {
		log.Error("Operation failed",
			"operation", work.operation,
			"schedule_id", work.scheduleID,
			"error", err)
		return err
	}
	return nil
}

// collectReconciliationErrors collects errors from the error channel
func (m *manager) collectReconciliationErrors(ctx context.Context, errChan <-chan error) error {
	log := logger.FromContext(ctx)
	var multiErr *MultiError
	for err := range errChan {
		multiErr = AppendError(multiErr, err)
	}
	if multiErr != nil && len(multiErr.Errors) > 0 {
		log.Error("Reconciliation encountered errors", "error_count", len(multiErr.Errors))
		return multiErr
	}
	return nil
}

// OnConfigurationReload handles workflow configuration reload events
func (m *manager) OnConfigurationReload(ctx context.Context, workflows []*workflow.Config) error {
	log := logger.FromContext(ctx).With("project", m.projectID)
	log.Info("Configuration reload detected, triggering schedule reconciliation")
	return m.ReconcileSchedules(ctx, workflows)
}

// StartPeriodicReconciliation starts a background goroutine for periodic reconciliation
func (m *manager) StartPeriodicReconciliation(
	ctx context.Context,
	getWorkflows func() []*workflow.Config,
	interval time.Duration,
) error {
	log := logger.FromContext(ctx).With("project", m.projectID)
	if interval <= 0 {
		return fmt.Errorf("periodic reconciliation interval must be positive, got %v", interval)
	}
	// Stop any existing periodic reconciliation
	m.StopPeriodicReconciliation()
	// Create new cancellation context
	periodicCtx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	m.periodicCancel = cancel
	m.mu.Unlock()
	// Start periodic reconciliation goroutine
	m.periodicWG.Add(1)
	go func() {
		defer m.periodicWG.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		log.Info("Started periodic schedule reconciliation", "interval", interval, "project_id", m.projectID)
		for {
			select {
			case <-periodicCtx.Done():
				log.Info("Stopping periodic schedule reconciliation", "project_id", m.projectID)
				return
			case <-ticker.C:
				workflows := getWorkflows()
				if err := m.ReconcileSchedules(periodicCtx, workflows); err != nil {
					log.Error("Periodic reconciliation failed", "error", err)
				}
			}
		}
	}()
	return nil
}

// StopPeriodicReconciliation stops the periodic reconciliation goroutine
func (m *manager) StopPeriodicReconciliation() {
	m.mu.Lock()
	if m.periodicCancel != nil {
		m.periodicCancel()
		m.periodicCancel = nil
	}
	m.mu.Unlock()
	// Wait for the goroutine to finish
	m.periodicWG.Wait()
}
