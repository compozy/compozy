package core

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/gin-gonic/gin"
)

const (
	loopActionRun     = "loop_run"
	loopActionPause   = "loop_pause"
	loopActionResume  = "loop_resume"
	loopActionApprove = "loop_approve"
	loopInvalidCursor = "invalid_cursor"
)

func (h *BaseHandlers) requireLoopService(c *gin.Context) (LoopService, bool) {
	if h.Loops == nil {
		h.respondError(
			c,
			http.StatusServiceUnavailable,
			fmt.Errorf("%s: loop service is not configured", h.transportName()),
		)
		return nil, false
	}
	return h.Loops, true
}

// RunLoop starts or dry-runs a Loop.
func (h *BaseHandlers) RunLoop(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	var req contract.RunLoopRequest
	if err := decodeStrictLoopJSONBody(c, &req); err != nil {
		h.respondLoopError(c, loopDecodeError("run", err))
		return
	}
	dry, err := ParseOptionalBool(c.Query("dry"))
	if err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: dry query: %v", looppkg.ErrValidation, err))
		return
	}
	mutationScope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	actor, err := h.taskActorContextForWorkspace(c, loopActionRun, c.Param("workspace_id"))
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	response, err := service.RunLoop(
		c.Request.Context(),
		c.Param("workspace_id"),
		c.Param("name"),
		LoopRunInput{
			Request: req, ProfileID: mutationScope.ProfileID,
			StartKind: loopStartKindForTransport(h.transportName()), Actor: actor, Dry: dry,
		},
	)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	if dry {
		c.JSON(http.StatusOK, response)
		return
	}
	c.JSON(http.StatusCreated, response)
}

// ListLoopRuns returns workspace-scoped Loop runs.
func (h *BaseHandlers) ListLoopRuns(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	query, err := ParseLoopRunListQuery(c)
	if err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: %v", looppkg.ErrValidation, err))
		return
	}
	query.ReadScope, err = h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	response, err := service.ListLoopRuns(c.Request.Context(), c.Param("workspace_id"), query)
	if err != nil {
		if errors.Is(err, looppkg.ErrInvalidRunListCursor) {
			c.JSON(http.StatusBadRequest, gin.H{bridgesErrorKey: loopInvalidCursor})
			return
		}
		h.respondLoopError(c, err)
		return
	}
	if err := h.decorateLoopRunOwners(c.Request.Context(), &response); err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// GetLoopRun returns one workspace-scoped Loop run.
func (h *BaseHandlers) GetLoopRun(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	readScope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	response, err := service.GetLoopRun(c.Request.Context(), c.Param("workspace_id"), c.Param("run_id"))
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	if !readScope.Matches(response.Run.ProfileID) {
		h.respondLoopError(c, looppkg.ErrRunNotFound)
		return
	}
	if err := h.decorateLoopRunOwner(c.Request.Context(), &response.Run); err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// PauseLoopRun requests operator pause for one Loop run.
func (h *BaseHandlers) PauseLoopRun(c *gin.Context) {
	h.mutateLoopRun(c, func(service LoopService) error {
		workspaceID := c.Param("workspace_id")
		actor, err := h.taskActorContextForWorkspace(c, loopActionPause, workspaceID)
		if err != nil {
			return err
		}
		return service.PauseLoopRun(c.Request.Context(), workspaceID, c.Param("run_id"), actor)
	})
}

// ResumeLoopRun resumes one paused Loop run.
func (h *BaseHandlers) ResumeLoopRun(c *gin.Context) {
	h.mutateLoopRun(c, func(service LoopService) error {
		workspaceID := c.Param("workspace_id")
		actor, err := h.taskActorContextForWorkspace(c, loopActionResume, workspaceID)
		if err != nil {
			return err
		}
		return service.ResumeLoopRun(c.Request.Context(), workspaceID, c.Param("run_id"), actor)
	})
}

// ApproveLoopRun applies a human-gate decision.
func (h *BaseHandlers) ApproveLoopRun(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	var req contract.ApproveLoopRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: decode approve loop request: %v", looppkg.ErrValidation, err))
		return
	}
	if !h.requireLoopRunProfile(c, service, true) {
		return
	}
	workspaceID := c.Param("workspace_id")
	actor, err := h.taskActorContextForWorkspace(c, loopActionApprove, workspaceID)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	if err := service.ApproveLoopRun(c.Request.Context(), workspaceID, c.Param("run_id"), req, actor); err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// StreamLoopRunEvents streams retained Loop run events over SSE.
func (h *BaseHandlers) StreamLoopRunEvents(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	if !h.requireLoopRunProfile(c, service, false) {
		return
	}
	readScope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	afterSeq, err := parseLastEventID(c.GetHeader("Last-Event-ID"), h.transportName())
	if err != nil {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("%w: %v", looppkg.ErrValidation, err))
		return
	}
	if querySeq, err := ParseOptionalInt64(c.Query("after_sequence")); err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: after_sequence query: %v", looppkg.ErrValidation, err))
		return
	} else if querySeq > afterSeq {
		afterSeq = querySeq
	}
	writer, err := PrepareSSE(c)
	if err != nil {
		h.logSSEWriteFailure("loop_events", err)
		return
	}

	pollInterval := h.PollInterval
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		events, err := service.ListLoopRunEvents(
			c.Request.Context(),
			c.Param("workspace_id"),
			c.Param("run_id"),
			afterSeq,
			readScope,
		)
		if err != nil {
			h.logSSEWriteFailure("loop_events", err)
			return
		}
		for _, event := range events {
			if event.Seq <= afterSeq {
				continue
			}
			afterSeq = event.Seq
			if err := WriteSSE(writer, SSEMessage{
				ID:   strconv.FormatInt(event.Seq, 10),
				Name: string(event.Kind),
				Data: event,
			}); err != nil {
				h.logSSEWriteFailure(string(event.Kind), err)
				return
			}
		}
		select {
		case <-c.Request.Context().Done():
			return
		case <-h.StreamDoneChannel():
			return
		case <-ticker.C:
		}
	}
}

func loopStartKindForTransport(transport string) dsl.StartKind {
	switch strings.TrimSpace(transport) {
	case transportNameUDSAPI:
		return dsl.StartUDS
	default:
		return dsl.StartHTTP
	}
}

func (h *BaseHandlers) mutateLoopRun(c *gin.Context, mutate func(LoopService) error) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	if !h.requireLoopRunProfile(c, service, true) {
		return
	}
	if err := mutate(service); err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *BaseHandlers) respondLoopError(c *gin.Context, err error) {
	if conflict, ok := errors.AsType[*LoopVersionConflictError](err); ok {
		c.JSON(http.StatusConflict, contract.LoopVersionConflictResponse{
			Error:          ErrLoopVersionConflict.Error(),
			CurrentVersion: conflict.CurrentVersion,
		})
		return
	}
	if lint, ok := errors.AsType[*LoopLintFailedError](err); ok {
		c.JSON(http.StatusUnprocessableEntity, contract.LoopValidationResponse{
			Valid:  false,
			Errors: lint.Errors,
		})
		return
	}
	if runtimeValidation, ok := errors.AsType[*looppkg.RuntimeValidationError](err); ok {
		items := make([]contract.LoopRuntimeValidationItemPayload, 0, len(runtimeValidation.Items))
		for _, item := range runtimeValidation.Items {
			items = append(items, contract.LoopRuntimeValidationItemPayload{
				TaskID: item.TaskID,
				Field:  item.Field,
				Value:  item.Value,
				Reason: item.Reason,
			})
		}
		c.JSON(http.StatusUnprocessableEntity, contract.LoopValidationResponse{
			Valid:             false,
			RuntimeValidation: items,
		})
		return
	}
	if inputValidation, ok := errors.AsType[*looppkg.InputValidationError](err); ok {
		c.JSON(http.StatusUnprocessableEntity, contract.LoopValidationResponse{
			Valid: false,
			InputValidation: &contract.LoopInputValidationErrorPayload{
				Loop: inputValidation.Loop, Field: inputValidation.Field,
				Kind: inputValidation.Kind, Value: inputValidation.Value,
				Origin: string(inputValidation.Origin), Reason: string(inputValidation.Reason),
			},
		})
		return
	}
	h.respondError(c, StatusForLoopError(err), err)
}
