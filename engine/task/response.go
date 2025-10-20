package task

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

// -----------------------------------------------------------------------------
// Response Interface - Common interface for task responses
// -----------------------------------------------------------------------------

type Response interface {
	GetState() *State
	GetOnSuccess() *core.SuccessTransition
	GetOnError() *core.ErrorTransition
	GetNextTask() *Config
	NextTaskID(ctx context.Context) string
}

// -----------------------------------------------------------------------------
// MainTaskResponse - For main task execution (basic, router, etc.)
// -----------------------------------------------------------------------------

type MainTaskResponse struct {
	OnSuccess *core.SuccessTransition `json:"on_success"`
	OnError   *core.ErrorTransition   `json:"on_error"`
	State     *State                  `json:"state"`
	NextTask  *Config                 `json:"next_task"`
}

func (r *MainTaskResponse) GetState() *State {
	return r.State
}

func (r *MainTaskResponse) GetOnSuccess() *core.SuccessTransition {
	return r.OnSuccess
}

func (r *MainTaskResponse) GetOnError() *core.ErrorTransition {
	return r.OnError
}

func (r *MainTaskResponse) GetNextTask() *Config {
	return r.NextTask
}

func (r *MainTaskResponse) NextTaskID(ctx context.Context) string {
	log := logger.FromContext(ctx)
	if r.State == nil {
		return ""
	}
	state := r.State
	taskID := state.TaskID
	var nextTaskID string
	switch {
	case state.Status == core.StatusSuccess && r.OnSuccess != nil && r.OnSuccess.Next != nil:
		nextTaskID = *r.OnSuccess.Next
		log.Info("Task succeeded, transitioning to next task",
			"current_task", taskID,
			"next_task", nextTaskID,
		)
	case state.Status == core.StatusFailed && r.OnError != nil && r.OnError.Next != nil:
		nextTaskID = *r.OnError.Next
		log.Info("Task failed, transitioning to error task",
			"current_task", taskID,
			"next_task", nextTaskID,
		)
	default:
		log.Info("No more tasks to execute", "current_task", taskID)
	}
	return nextTaskID
}

// -----------------------------------------------------------------------------
// SubtaskResponse - For parallel subtask execution
// -----------------------------------------------------------------------------

type SubtaskResponse struct {
	TaskID string          `json:"task_id"`
	Output *core.Output    `json:"output"`
	Error  *core.Error     `json:"error"`
	Status core.StatusType `json:"status"`
	State  *State          `json:"state"`
}

func (r *SubtaskResponse) GetState() *State {
	return r.State
}

func (r *SubtaskResponse) GetOnSuccess() *core.SuccessTransition {
	// Subtasks don't have transitions
	return nil
}

func (r *SubtaskResponse) GetOnError() *core.ErrorTransition {
	// Subtasks don't have transitions
	return nil
}

func (r *SubtaskResponse) GetNextTask() *Config {
	// Subtasks don't have next tasks
	return nil
}

func (r *SubtaskResponse) NextTaskID(_ context.Context) string {
	// Subtasks don't transition to other tasks
	return ""
}

// -----------------------------------------------------------------------------
// CollectionResponse - For collection task execution
// -----------------------------------------------------------------------------

type CollectionResponse struct {
	*MainTaskResponse     // Embedded main task response
	ItemCount         int `json:"item_count"`    // Number of items processed
	SkippedCount      int `json:"skipped_count"` // Number of items filtered out
}

func (r *CollectionResponse) GetState() *State {
	return r.MainTaskResponse.GetState()
}

func (r *CollectionResponse) GetOnSuccess() *core.SuccessTransition {
	return r.MainTaskResponse.GetOnSuccess()
}

func (r *CollectionResponse) GetOnError() *core.ErrorTransition {
	return r.MainTaskResponse.GetOnError()
}

func (r *CollectionResponse) GetNextTask() *Config {
	return r.MainTaskResponse.GetNextTask()
}

func (r *CollectionResponse) NextTaskID(ctx context.Context) string {
	return r.MainTaskResponse.NextTaskID(ctx)
}
