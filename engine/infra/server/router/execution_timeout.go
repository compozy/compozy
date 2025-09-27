package router

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

type TimeoutStateEncoder func(*task.State) any

type TimeoutResponseOptions struct {
	ResourceKind  string
	ResourceID    string
	ExecutionKind string
	Metrics       *monitoring.ExecutionMetrics
}

func RespondExecutionTimeout(
	c *gin.Context,
	repo task.Repository,
	execID core.ID,
	encode TimeoutStateEncoder,
	opts TimeoutResponseOptions,
) int {
	requestCtx := c.Request.Context()
	stateCtx := context.WithoutCancel(requestCtx)
	log := logger.FromContext(requestCtx)
	payload := gin.H{"exec_id": execID.String()}
	if repo != nil {
		state, err := repo.GetState(stateCtx, execID)
		if err == nil {
			if encode != nil {
				if dto := encode(state); dto != nil {
					payload["state"] = dto
				}
			}
		} else if !errors.Is(err, store.ErrTaskNotFound) {
			log.Warn(
				fmt.Sprintf("Failed to load %s execution state after timeout", strings.ToLower(opts.ResourceKind)),
				"resource_id", opts.ResourceID,
				"exec_id", execID.String(),
				"error", err,
			)
		}
	}
	const timeoutMessage = "execution timeout"
	log.Warn(
		fmt.Sprintf("%s execution timed out", opts.ResourceKind),
		"resource_id", opts.ResourceID,
		"exec_id", execID.String(),
	)
	resp := Response{
		Status:  http.StatusRequestTimeout,
		Message: timeoutMessage,
		Data:    payload,
		Error: &ErrorInfo{
			Code:    ErrRequestTimeoutCode,
			Message: timeoutMessage,
			Details: context.DeadlineExceeded.Error(),
		},
	}
	c.JSON(http.StatusRequestTimeout, resp)
	if opts.Metrics != nil && opts.ExecutionKind != "" {
		opts.Metrics.RecordTimeout(requestCtx, opts.ExecutionKind)
	}
	return http.StatusRequestTimeout
}
