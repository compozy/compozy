package shared

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

// ProcessResult contains the result of processing a template
type ProcessResult struct {
	Text string
	YAML any
	JSON any
}

// TaskNormalizer defines the contract for task-specific normalization
type TaskNormalizer interface {
	// Normalize applies task-specific normalization rules
	Normalize(config any, ctx *NormalizationContext) error
	// Type returns the task type this normalizer handles
	Type() string
}

// NormalizerFactory creates appropriate normalizers
type NormalizerFactory interface {
	CreateNormalizer(taskType string) (TaskNormalizer, error)
}

// -----------------------------------------------------------------------------
// New Response Handler Interfaces
// -----------------------------------------------------------------------------

// ResponseInput contains the input data for response handlers
type ResponseInput struct {
	TaskConfig     *task.Config     `json:"task_config"`
	TaskState      *task.State      `json:"task_state"`
	WorkflowConfig *workflow.Config `json:"workflow_config"`
	WorkflowState  *workflow.State  `json:"workflow_state"`
	ExecutionError error            `json:"execution_error,omitempty"`
}

// ResponseOutput contains the output data from response handlers
type ResponseOutput struct {
	Response any         `json:"response"`
	State    *task.State `json:"state"`
}

// TaskResponseHandler defines the contract for task-specific response handling
type TaskResponseHandler interface {
	// HandleResponse processes a task execution response
	HandleResponse(ctx context.Context, input *ResponseInput) (*ResponseOutput, error)
	// Type returns the task type this handler processes
	Type() task.Type
}

// ParentStatusManager handles parent task status aggregation
type ParentStatusManager interface {
	// UpdateParentStatus updates parent task status based on child completion
	UpdateParentStatus(ctx context.Context, parentStateID core.ID, strategy task.ParallelStrategy) error
	// GetAggregatedStatus calculates the aggregated status for a parent task
	GetAggregatedStatus(
		ctx context.Context,
		parentStateID core.ID,
		strategy task.ParallelStrategy,
	) (core.StatusType, error)
}

// -----------------------------------------------------------------------------
// Domain Service Interfaces
// -----------------------------------------------------------------------------

// ExpansionResult contains the result of collection item expansion
type ExpansionResult struct {
	ChildConfigs []*task.Config `json:"child_configs"`
	ItemCount    int            `json:"item_count"`
	SkippedCount int            `json:"skipped_count"`
}

// CollectionExpander handles collection task item expansion and child config creation
type CollectionExpander interface {
	// ExpandItems expands collection items into child task configurations
	ExpandItems(
		ctx context.Context,
		config *task.Config,
		workflowState *workflow.State,
		workflowConfig *workflow.Config,
	) (*ExpansionResult, error)
	// ValidateExpansion validates the expansion result
	ValidateExpansion(result *ExpansionResult) error
}

// -----------------------------------------------------------------------------
// Infrastructure Service Interfaces
// -----------------------------------------------------------------------------

// TaskConfigRepository handles storage and retrieval of task configuration data
type TaskConfigRepository interface {
	// Parallel Task Methods
	StoreParallelMetadata(ctx context.Context, parentStateID core.ID, metadata any) error
	LoadParallelMetadata(ctx context.Context, parentStateID core.ID) (any, error)

	// Collection Task Methods
	StoreCollectionMetadata(ctx context.Context, parentStateID core.ID, metadata any) error
	LoadCollectionMetadata(ctx context.Context, parentStateID core.ID) (any, error)

	// Composite Task Methods
	StoreCompositeMetadata(ctx context.Context, parentStateID core.ID, metadata any) error
	LoadCompositeMetadata(ctx context.Context, parentStateID core.ID) (any, error)

	// Generic Task Config Methods
	SaveTaskConfig(ctx context.Context, taskExecID string, config *task.Config) error
	GetTaskConfig(ctx context.Context, taskExecID string) (*task.Config, error)
	DeleteTaskConfig(ctx context.Context, taskExecID string) error

	// Strategy Management Methods
	ExtractParallelStrategy(ctx context.Context, parentState *task.State) (task.ParallelStrategy, error)
	ValidateStrategy(strategy string) (task.ParallelStrategy, error)
	CalculateMaxWorkers(taskType task.Type, maxWorkers int) int
}
