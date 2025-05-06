package task

import (
	"encoding/json"
)

type TaskID string
type TaskName string
type TaskTimeout string
type TaskCondition string
type TaskRoute string
type TaskMode string
type TaskInput string
type TaskCheckInterval string
type TaskErrorHandling string
type TaskFinal string
type TaskMaxConcurrent uint32
type TaskBatchSize uint32
type TaskMaxChecks uint32
type MappingValues map[string]json.RawMessage
