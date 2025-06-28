package shared

// Field names used throughout the task2 package for consistent map key access
const (
	InputKey    = "input"
	OutputKey   = "output"
	StatusKey   = "status"
	ErrorKey    = "error"
	IDKey       = "id"
	WorkflowKey = "workflow"
	TasksKey    = "tasks"
	ParentKey   = "parent"
	ItemKey     = "item"
	IndexKey    = "index"
	EnvKey      = "env"
	ChildrenKey = "children"
	ActionKey   = "action"
	TypeKey     = "type"
	WithKey     = "with"
	AgentKey    = "agent"
	ToolKey     = "tool"
	OutputsKey  = "outputs"
)

// Default recursion and depth limits for preventing infinite loops and memory exhaustion
// These can be overridden by project configuration or environment variables
const (
	DefaultMaxContextDepth  = 10       // Default maximum depth for recursive context building
	DefaultMaxParentDepth   = 10       // Default maximum depth for parent chain traversal
	DefaultMaxChildrenDepth = 10       // Default maximum depth for children context building
	DefaultMaxConfigDepth   = 10       // Default maximum depth for configuration validation
	DefaultMaxTemplateDepth = 5        // Default maximum depth for template processing
	DefaultMaxStringLength  = 10485760 // 10MB default for input sanitization

	// Environment variable names for configurable limits
	EnvMaxNestingDepth     = "MAX_NESTING_DEPTH"
	EnvMaxStringLength     = "MAX_STRING_LENGTH"
	EnvMaxTaskContextDepth = "COMPOZY_MAX_TASK_CONTEXT_DEPTH"
)
