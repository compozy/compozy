package services

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/core"
)

// WorkflowService defines read-only operations for workflow management
type WorkflowService interface {
	List(ctx context.Context, filters WorkflowFilters) ([]Workflow, error)
	Get(ctx context.Context, id core.ID) (*WorkflowDetail, error)
}

// WorkflowMutateService defines mutate operations for workflow management
type WorkflowMutateService interface {
	Execute(ctx context.Context, id core.ID, input ExecutionInput) (*ExecutionResult, error)
}

// ExecutionService defines read-only operations for execution management
type ExecutionService interface {
	List(ctx context.Context, filters ExecutionFilters) ([]Execution, error)
	Get(ctx context.Context, id core.ID) (*ExecutionDetail, error)
	Follow(ctx context.Context, id core.ID) (<-chan ExecutionEvent, error)
}

// ExecutionMutateService defines mutate operations for execution management
type ExecutionMutateService interface {
	Signal(ctx context.Context, execID core.ID, signal string, payload any) error
	Cancel(ctx context.Context, execID core.ID) error
}

// ScheduleService defines read-only operations for schedule management
type ScheduleService interface {
	List(ctx context.Context) ([]Schedule, error)
	Get(ctx context.Context, workflowID core.ID) (*Schedule, error)
}

// ScheduleMutateService defines mutate operations for schedule management
type ScheduleMutateService interface {
	Update(ctx context.Context, workflowID core.ID, req UpdateScheduleRequest) error
	Delete(ctx context.Context, workflowID core.ID) error
}

// EventService defines operations for event management
type EventService interface {
	Send(ctx context.Context, event Event) error
}

// Workflow represents a workflow definition
type Workflow struct {
	ID          core.ID           `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Status      WorkflowStatus    `json:"status"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Tags        []string          `json:"tags"`
	Metadata    map[string]string `json:"metadata"`
}

// WorkflowDetail represents detailed workflow information
type WorkflowDetail struct {
	Workflow
	Tasks      []Task         `json:"tasks"`
	Inputs     []InputSchema  `json:"inputs"`
	Outputs    []OutputSchema `json:"outputs"`
	Schedule   *Schedule      `json:"schedule,omitempty"`
	Statistics *WorkflowStats `json:"statistics"`
}

// WorkflowStatus represents workflow status
type WorkflowStatus string

const (
	WorkflowStatusActive   WorkflowStatus = "active"
	WorkflowStatusInactive WorkflowStatus = "inactive"
	WorkflowStatusDeleted  WorkflowStatus = "deleted"
)

// WorkflowFilters represents filters for workflow listing
type WorkflowFilters struct {
	Status string   `json:"status,omitempty"`
	Tags   []string `json:"tags,omitempty"`
	Limit  int      `json:"limit,omitempty"`
	Offset int      `json:"offset,omitempty"`
}

// Execution represents a workflow execution
type Execution struct {
	ID          core.ID         `json:"id"`
	WorkflowID  core.ID         `json:"workflow_id"`
	Status      ExecutionStatus `json:"status"`
	StartedAt   time.Time       `json:"started_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	Duration    *time.Duration  `json:"duration,omitempty"`
	Input       any             `json:"input,omitempty"`
	Output      any             `json:"output,omitempty"`
	Error       *ExecutionError `json:"error,omitempty"`
}

// ExecutionDetail represents detailed execution information
type ExecutionDetail struct {
	Execution
	Logs        []LogEntry        `json:"logs"`
	TaskResults []TaskResult      `json:"task_results"`
	Metrics     *ExecutionMetrics `json:"metrics"`
}

// ExecutionStatus represents execution status
type ExecutionStatus string

const (
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusCancelled ExecutionStatus = "canceled"
)

// ExecutionFilters represents filters for execution listing
type ExecutionFilters struct {
	WorkflowID core.ID         `json:"workflow_id,omitempty"`
	Status     ExecutionStatus `json:"status,omitempty"`
	Limit      int             `json:"limit,omitempty"`
	Offset     int             `json:"offset,omitempty"`
}

// ExecutionInput represents input for workflow execution
type ExecutionInput struct {
	Data     any               `json:"data,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ExecutionResult represents the result of workflow execution
type ExecutionResult struct {
	ExecutionID core.ID         `json:"execution_id"`
	Status      ExecutionStatus `json:"status"`
	Message     string          `json:"message,omitempty"`
}

// ExecutionEvent represents real-time execution events
type ExecutionEvent struct {
	ExecutionID core.ID   `json:"execution_id"`
	Type        string    `json:"type"`
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
	Data        any       `json:"data,omitempty"`
}

// Schedule represents a workflow schedule
type Schedule struct {
	WorkflowID core.ID    `json:"workflow_id"`
	CronExpr   string     `json:"cron_expression"`
	Enabled    bool       `json:"enabled"`
	NextRun    time.Time  `json:"next_run"`
	LastRun    *time.Time `json:"last_run,omitempty"`
	Timezone   string     `json:"timezone"`
}

// UpdateScheduleRequest represents a request to update a schedule
type UpdateScheduleRequest struct {
	CronExpr *string `json:"cron_expression,omitempty"`
	Enabled  *bool   `json:"enabled,omitempty"`
	Timezone *string `json:"timezone,omitempty"`
}

// Event represents an event to be sent
type Event struct {
	Name      string    `json:"name"`
	Payload   any       `json:"payload,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"`
}

// Supporting types
type Task struct {
	ID          core.ID `json:"id"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
}

type InputSchema struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

type OutputSchema struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

type WorkflowStats struct {
	TotalExecutions      int64         `json:"total_executions"`
	SuccessfulExecutions int64         `json:"successful_executions"`
	FailedExecutions     int64         `json:"failed_executions"`
	AverageExecutionTime time.Duration `json:"average_execution_time"`
}

type ExecutionError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	TaskID    *core.ID  `json:"task_id,omitempty"`
}

type TaskResult struct {
	TaskID      core.ID         `json:"task_id"`
	Status      string          `json:"status"`
	Output      any             `json:"output,omitempty"`
	Error       *ExecutionError `json:"error,omitempty"`
	StartedAt   time.Time       `json:"started_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}

type ExecutionMetrics struct {
	TotalTasks     int            `json:"total_tasks"`
	CompletedTasks int            `json:"completed_tasks"`
	FailedTasks    int            `json:"failed_tasks"`
	ExecutionTime  time.Duration  `json:"execution_time"`
	ResourceUsage  *ResourceUsage `json:"resource_usage,omitempty"`
}

type ResourceUsage struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage int64   `json:"memory_usage"`
	DiskUsage   int64   `json:"disk_usage"`
}
