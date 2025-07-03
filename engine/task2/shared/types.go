package shared

import (
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
)

// -----------------------------------------------------------------------------
// Response Handler Types
// -----------------------------------------------------------------------------

// DeferredOutputConfig controls when output transformation should be deferred
type DeferredOutputConfig struct {
	ShouldDefer bool   `json:"should_defer"`
	Reason      string `json:"reason,omitempty"`
}

// ResponseContext provides additional context for response handling
type ResponseContext struct {
	ParentTaskID   string                `json:"parent_task_id,omitempty"`
	ChildIndex     int                   `json:"child_index,omitempty"`
	IsParentTask   bool                  `json:"is_parent_task"`
	DeferredConfig *DeferredOutputConfig `json:"deferred_config,omitempty"`
}

// ValidationResult contains the result of validation operations
type ValidationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// -----------------------------------------------------------------------------
// Collection Processing Types
// -----------------------------------------------------------------------------

// CollectionContext contains context variables for collection processing
type CollectionContext struct {
	Item     any    `json:"item"`
	Index    int    `json:"index"`
	ItemVar  string `json:"item_var,omitempty"`
	IndexVar string `json:"index_var,omitempty"`
}

// FilterResult contains the result of collection filtering
type FilterResult struct {
	FilteredItems []any `json:"filtered_items"`
	SkippedCount  int   `json:"skipped_count"`
	TotalCount    int   `json:"total_count"`
}

// -----------------------------------------------------------------------------
// Status Management Types
// -----------------------------------------------------------------------------

// StatusAggregationRule defines how child statuses should be aggregated
type StatusAggregationRule struct {
	Strategy    string `json:"strategy"` // "all_success", "any_success", "fail_fast"
	AllowFailed bool   `json:"allow_failed"`
}

// ParentStatusUpdate contains information for parent status updates
type ParentStatusUpdate struct {
	ParentStateID core.ID         `json:"parent_state_id"`
	ChildStatus   core.StatusType `json:"child_status"`
	ChildTaskID   string          `json:"child_task_id"`
	UpdatedAt     int64           `json:"updated_at"`
}

// -----------------------------------------------------------------------------
// Configuration Storage Types
// -----------------------------------------------------------------------------

// ConfigStorageMetadata contains metadata for configuration storage
type ConfigStorageMetadata struct {
	Version   string `json:"version"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// CompositeTaskMetadata contains metadata for composite task execution
type CompositeTaskMetadata struct {
	TaskConfigs []*task.Config         `json:"task_configs"`
	Metadata    *ConfigStorageMetadata `json:"metadata"`
}

// -----------------------------------------------------------------------------
// Error Types
// -----------------------------------------------------------------------------

// HandlerError represents an error from a response handler
type HandlerError struct {
	HandlerType string `json:"handler_type"`
	TaskType    string `json:"task_type"`
	Message     string `json:"message"`
	Cause       error  `json:"cause,omitempty"`
}

func (e *HandlerError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *HandlerError) Unwrap() error {
	return e.Cause
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

func (e *ValidationError) Error() string {
	if e.Value != "" {
		return e.Field + ": " + e.Message + " (value: " + e.Value + ")"
	}
	return e.Field + ": " + e.Message
}
