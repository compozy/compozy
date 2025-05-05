package task

import (
	"encoding/json"
)

// TaskID represents a task identifier
type TaskID string

// TaskName represents a task name
type TaskName string

// TaskTimeout represents a task timeout
type TaskTimeout string

// TaskCondition represents a task condition
type TaskCondition string

// TaskRoute represents a task route
type TaskRoute string

// TaskMode represents a task mode
type TaskMode string

// TaskInput represents a task input
type TaskInput string

// TaskCheckInterval represents a task check interval
type TaskCheckInterval string

// TaskErrorHandling represents a task error handling
type TaskErrorHandling string

// TaskFinal represents a task final state
type TaskFinal string

// TaskMaxConcurrent represents a task's maximum concurrent executions
type TaskMaxConcurrent uint32

// TaskBatchSize represents a task's batch size
type TaskBatchSize uint32

// TaskMaxChecks represents a task's maximum number of checks
type TaskMaxChecks uint32

// MappingValues represents a map of string keys to JSON values
type MappingValues map[string]json.RawMessage
