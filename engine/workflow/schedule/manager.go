package schedule

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/robfig/cron/v3"
	"github.com/sethvargo/go-retry"
	"go.opentelemetry.io/otel/metric"
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
	WorkflowID string         `json:"workflow_id"`
	ModifiedAt time.Time      `json:"modified_at"`
	Values     map[string]any `json:"values"`
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

// manager implements the Manager interface
type manager struct {
	client    *worker.Client
	projectID string
	taskQueue string
	mu        sync.RWMutex
	// Track API overrides with persistence and timestamp tracking
	overrideCache *OverrideCache
	// Metrics for observability
	metrics            *Metrics
	lastScheduleCounts map[string]int
	// Periodic reconciliation support
	periodicCancel context.CancelFunc
	periodicWG     sync.WaitGroup
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

// GetOverride retrieves an override for a workflow
func (c *OverrideCache) GetOverride(workflowID string) (*Override, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	override, exists := c.overrides[workflowID]
	if !exists {
		return nil, false
	}
	// Return a copy to prevent concurrent modification
	return &Override{
		WorkflowID: override.WorkflowID,
		ModifiedAt: override.ModifiedAt,
		Values:     copyValues(override.Values),
	}, true
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
		result[k] = &Override{
			WorkflowID: v.WorkflowID,
			ModifiedAt: v.ModifiedAt,
			Values:     copyValues(v.Values),
		}
	}
	return result
}

// copyValues creates a deep copy of the values map to prevent concurrent modification.
// It explicitly handles string slices, any slices, and nested maps.
// For all other types (primitives, pointers to primitives, etc.), a shallow copy is performed.
// IMPORTANT: If any new *mutable* types (e.g., custom structs, slices of non-primitive types)
// are ever stored in the 'Values' map, this function MUST be updated to deep copy them,
// otherwise concurrent modification issues may arise.
func copyValues(original map[string]any) map[string]any {
	if original == nil {
		return nil
	}
	result := make(map[string]any, len(original))
	for k, v := range original {
		// Deep copy based on type
		switch val := v.(type) {
		case []string:
			// Deep copy string slices
			copied := make([]string, len(val))
			copy(copied, val)
			result[k] = copied
		case []any:
			// Deep copy any slices
			copied := make([]any, len(val))
			copy(copied, val)
			result[k] = copied
		case map[string]any:
			// Recursively deep copy nested maps
			result[k] = copyValues(val)
		default:
			// For primitive types (bool, string, int, etc.) and their pointers,
			// shallow copy is sufficient
			result[k] = v
		}
	}
	return result
}

// getYAMLModTime gets the modification time of a workflow's YAML file
// Returns time.Now() on errors to force reconciliation, preventing overrides
// from becoming permanently stuck when file access fails temporarily.
func (m *manager) getYAMLModTime(ctx context.Context, wf *workflow.Config) time.Time {
	filePath := wf.GetFilePath()
	if filePath == "" {
		return time.Now() // Return current time to force reconciliation
	}
	stat, err := os.Stat(filePath)
	if err != nil {
		log := logger.FromContext(ctx)
		// Use Error level for better visibility of persistent file access issues
		log.Error(
			"Failed to get file modification time, forcing reconciliation",
			"workflow_id",
			wf.ID,
			"path",
			filePath,
			"error",
			err,
		)
		return time.Now() // Return current time on error to force reconciliation (fail-open approach)
	}
	return stat.ModTime()
}

// NewManager creates a new schedule manager
func NewManager(client *worker.Client, projectID string) Manager {
	return &manager{
		client:             client,
		projectID:          projectID,
		taskQueue:          slugify(projectID),
		overrideCache:      NewOverrideCache(),
		metrics:            nil, // Will be set by SetMetrics if monitoring is enabled
		lastScheduleCounts: make(map[string]int),
	}
}

// NewManagerWithMetrics creates a new schedule manager with metrics
func NewManagerWithMetrics(ctx context.Context, client *worker.Client, projectID string, meter metric.Meter) Manager {
	return &manager{
		client:             client,
		projectID:          projectID,
		taskQueue:          slugify(projectID),
		overrideCache:      NewOverrideCache(),
		metrics:            NewMetrics(ctx, meter),
		lastScheduleCounts: make(map[string]int),
	}
}

// ReconcileSchedules performs stateless reconciliation between workflows and Temporal schedules
func (m *manager) ReconcileSchedules(ctx context.Context, workflows []*workflow.Config) error {
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
		return err
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
func (m *manager) buildReconciliationState(ctx context.Context, workflows []*workflow.Config) (
	map[string]client.ScheduleHandle, map[string]*workflow.Config, map[string]time.Time, error) {
	log := logger.FromContext(ctx)

	// Get all existing schedules from Temporal
	existingSchedules, err := m.listSchedulesByPrefix(ctx, m.schedulePrefix())
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to list existing schedules: %w", err)
	}
	log.Debug("Found existing schedules", "count", len(existingSchedules))

	// Build desired state from YAML
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

	m.mu.Lock()
	defer m.mu.Unlock()

	// Calculate and report deltas - combine keys from both maps
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
		if delta != 0 {
			m.metrics.UpdateWorkflowCount(ctx, m.projectID, status, delta)
		}
	}
	m.lastScheduleCounts = currentCounts
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
		if strings.Contains(err.Error(), "workflow not found") || strings.Contains(err.Error(), "not found") {
			return nil, ErrScheduleNotFound
		}
		return nil, fmt.Errorf("failed to get schedule for workflow %s: %w", workflowID, err)
	}
	return info, nil
}

// UpdateSchedule updates a schedule (for temporary overrides)
func (m *manager) UpdateSchedule(ctx context.Context, workflowID string, update UpdateRequest) error {
	log := logger.FromContext(ctx).With("workflow_id", workflowID, "project", m.projectID)
	scheduleID := m.scheduleID(workflowID)
	handle := m.client.ScheduleClient().GetHandle(ctx, scheduleID)

	// Get current schedule description to store original values
	desc, err := handle.Describe(ctx)
	if err != nil {
		// Check if this is a "not found" error
		if strings.Contains(err.Error(), "workflow not found") || strings.Contains(err.Error(), "not found") {
			return ErrScheduleNotFound
		}
		return fmt.Errorf("failed to describe schedule before update: %w", err)
	}

	// Log API override operation
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

	// Prepare override values with original YAML values
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
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if _, err := parser.Parse(*update.Cron); err != nil {
			return fmt.Errorf("invalid cron expression '%s': %w", *update.Cron, err)
		}
		values["cron"] = *update.Cron
	}

	// Store the override in cache
	m.overrideCache.SetOverride(workflowID, values)

	// Update in Temporal
	err = handle.Update(ctx, client.ScheduleUpdateOptions{
		DoUpdate: func(schedule client.ScheduleUpdateInput) (*client.ScheduleUpdate, error) {
			if update.Enabled != nil {
				schedule.Description.Schedule.State.Paused = !*update.Enabled
			}
			if update.Cron != nil {
				schedule.Description.Schedule.Spec.CronExpressions = []string{*update.Cron}
			}
			return &client.ScheduleUpdate{
				Schedule: &schedule.Description.Schedule,
			}, nil
		},
	})
	if err != nil {
		// Remove override on failure
		m.overrideCache.ClearOverride(workflowID)
		return fmt.Errorf("failed to update schedule %s: %w", workflowID, err)
	}
	return nil
}

// DeleteSchedule removes a schedule from Temporal
func (m *manager) DeleteSchedule(ctx context.Context, workflowID string) error {
	scheduleID := m.scheduleID(workflowID)
	handle := m.client.ScheduleClient().GetHandle(ctx, scheduleID)
	err := handle.Delete(ctx)
	if err != nil {
		// Check if this is a "not found" error
		if strings.Contains(err.Error(), "workflow not found") || strings.Contains(err.Error(), "not found") {
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
	if strings.HasPrefix(scheduleID, prefix) {
		return strings.TrimPrefix(scheduleID, prefix)
	}
	return ""
}

// slugify converts a string to a valid Temporal task queue name
func slugify(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, " ", "-"))
}

// executeReconciliation performs the actual reconciliation operations
func (m *manager) executeReconciliation(
	ctx context.Context,
	toCreate, toUpdate map[string]*workflow.Config,
	toDelete []string,
) error {
	log := logger.FromContext(ctx)
	// Use a semaphore to limit concurrent operations
	const maxConcurrent = 10
	sem := make(chan struct{}, maxConcurrent)
	// Error channel to collect errors from goroutines
	errChan := make(chan error, len(toCreate)+len(toUpdate)+len(toDelete))
	var wg sync.WaitGroup
	// Helper function to execute an operation with rate limiting and retry
	executeOp := func(op func() error) {
		defer wg.Done()
		// Acquire semaphore
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
		case <-ctx.Done():
			errChan <- ctx.Err()
			return
		}
		// Execute operation with retry
		backoff := retry.WithMaxRetries(3, retry.NewExponential(1*time.Second))
		err := retry.Do(ctx, backoff, func(_ context.Context) error {
			return op()
		})
		if err != nil {
			errChan <- err
		}
	}
	// Create schedules
	for scheduleID, wf := range toCreate {
		scheduleID, wf := scheduleID, wf // Capture loop variables
		wg.Add(1)
		go executeOp(func() error {
			if err := m.createSchedule(ctx, scheduleID, wf); err != nil {
				return fmt.Errorf("failed to create schedule %s: %w", scheduleID, err)
			}
			return nil
		})
	}
	// Update schedules
	for scheduleID, wf := range toUpdate {
		scheduleID, wf := scheduleID, wf // Capture loop variables
		wg.Add(1)
		go executeOp(func() error {
			if err := m.updateSchedule(ctx, scheduleID, wf); err != nil {
				return fmt.Errorf("failed to update schedule %s: %w", scheduleID, err)
			}
			return nil
		})
	}
	// Delete schedules
	for _, scheduleID := range toDelete {
		scheduleID := scheduleID // Capture loop variable
		wg.Add(1)
		go executeOp(func() error {
			if err := m.deleteSchedule(ctx, scheduleID); err != nil {
				return fmt.Errorf("failed to delete schedule %s: %w", scheduleID, err)
			}
			return nil
		})
	}
	// Wait for all operations to complete
	wg.Wait()
	close(errChan)
	// Collect any errors
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
				// Fetch the latest workflows on each tick
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
