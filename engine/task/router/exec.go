package tkrouter

import (
	"errors"
	"net/http"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/task"
	"github.com/gin-gonic/gin"
)

func getTaskExecutionStatus(c *gin.Context) {
	execID := router.GetTaskExecID(c)
	if execID == "" {
		return
	}
	state := router.GetAppState(c)
	if state == nil {
		return
	}
	repo := router.ResolveTaskRepository(c, state)
	if repo == nil {
		reqErr := router.NewRequestError(http.StatusInternalServerError, "task repository unavailable", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	ctx := c.Request.Context()
	taskState, err := repo.GetState(ctx, execID)
	if err != nil {
		if errors.Is(err, store.ErrTaskNotFound) {
			reqErr := router.NewRequestError(http.StatusNotFound, "execution not found", err)
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
			return
		}
		reqErr := router.NewRequestError(http.StatusInternalServerError, "failed to load execution", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	resp := newTaskExecutionStatusDTO(taskState)
	router.RespondOK(c, "execution status retrieved", resp)
}

func newTaskExecutionStatusDTO(state *task.State) TaskExecutionStatusDTO {
	dto := TaskExecutionStatusDTO{
		ExecID:         state.TaskExecID.String(),
		Status:         state.Status,
		Component:      state.Component,
		TaskID:         state.TaskID,
		WorkflowID:     state.WorkflowID,
		WorkflowExecID: state.WorkflowExecID.String(),
		CreatedAt:      state.CreatedAt,
		UpdatedAt:      state.UpdatedAt,
	}
	if state.Output != nil {
		outputCopy := *state.Output
		dto.Output = &outputCopy
	}
	if state.Error != nil {
		errorCopy := *state.Error
		dto.Error = &errorCopy
	}
	return dto
}

type TaskExecutionStatusDTO struct {
	ExecID         string             `json:"exec_id"`
	Status         core.StatusType    `json:"status"`
	Component      core.ComponentType `json:"component"`
	TaskID         string             `json:"task_id"`
	WorkflowID     string             `json:"workflow_id"`
	WorkflowExecID string             `json:"workflow_exec_id"`
	Output         *core.Output       `json:"output,omitempty"`
	Error          *core.Error        `json:"error,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}
