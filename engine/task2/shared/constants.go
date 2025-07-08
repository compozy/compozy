package shared

// Field names used across response handlers
const (
	// Config field names
	AgentKey   = "agent"
	ToolKey    = "tool"
	TasksKey   = "tasks"
	OutputsKey = "outputs"
	InputKey   = "input"
	OutputKey  = "output"

	// Strategy field names
	FieldStrategy       = "strategy"
	FieldParallelConfig = "parallel_config"

	// Collection field names
	FieldCollectionItem     = "_collection_item"
	FieldCollectionItemVar  = "_collection_item_var"
	FieldCollectionIndex    = "_collection_index"
	FieldCollectionIndexVar = "_collection_index_var"

	// Router field names
	FieldRouteTaken = "route_taken"

	// Aggregate field names
	FieldAggregated = "aggregated"

	// Signal field names
	FieldSignal = "signal"

	// Common field names
	FieldItem  = "item"
	FieldIndex = "index"
)

// Error messages
const (
	ErrInputNil          = "Task input is missing. Please provide valid input data."
	ErrTaskConfigNil     = "Task configuration is missing. Please check your task definition."
	ErrTaskStateNil      = "Task state information is missing. This may indicate a system issue."
	ErrWorkflowConfigNil = "Workflow configuration is missing. Please check your workflow definition."
	ErrWorkflowStateNil  = "Workflow state information is missing. This may indicate a system issue."
	ErrInvalidTaskType   = "Task type mismatch. The task handler doesn't support this task type."
	ErrInvalidID         = "Invalid identifier provided. Please use a valid UUID format."
)

// Default limits
const (
	DefaultMaxParentDepth   = 10
	DefaultMaxStringLength  = 1024
	DefaultMaxContextDepth  = 5
	DefaultMaxChildrenDepth = 10
	DefaultMaxConfigDepth   = 10
	DefaultMaxTemplateDepth = 10
	DefaultBatchSize        = 100 // Default batch size for parent updates
)

// Environment variables - REMOVED: Now using pkg/config with COMPOZY_ prefixed env vars

// Context keys
const (
	IDKey       = "id"
	StatusKey   = "status"
	ErrorKey    = "error"
	ChildrenKey = "children"
	TypeKey     = "type"
	ActionKey   = "action"
	WithKey     = "with"
	EnvKey      = "env"
	ParentKey   = "parent"
	ItemKey     = "item"
	IndexKey    = "index"
	WorkflowKey = "workflow"
)
