package core

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/gin-gonic/gin"
)

const (
	loopActionRun     = "loop_run"
	loopActionStop    = "loop_stop"
	loopActionPause   = "loop_pause"
	loopActionResume  = "loop_resume"
	loopActionApprove = "loop_approve"
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

// CreateLoop creates or forks one writable Loop definition.
func (h *BaseHandlers) CreateLoop(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	var req contract.CreateLoopRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: decode create loop request: %v", looppkg.ErrValidation, err))
		return
	}
	response, err := service.CreateLoop(c.Request.Context(), c.Param("workspace_id"), req)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusCreated, response)
}

// GetLoop inspects one resolved Loop definition.
func (h *BaseHandlers) GetLoop(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	response, err := service.GetLoop(c.Request.Context(), c.Param("workspace_id"), c.Param("name"))
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// PatchLoop atomically lints, compiles, and publishes one Loop definition.
func (h *BaseHandlers) PatchLoop(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	var req contract.PatchLoopRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: decode patch loop request: %v", looppkg.ErrValidation, err))
		return
	}
	response, err := service.PatchLoop(c.Request.Context(), c.Param("workspace_id"), c.Param("name"), req)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// ValidateLoop lints and compiles one Loop definition without saving it.
func (h *BaseHandlers) ValidateLoop(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	var req contract.ValidateLoopRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: decode validate loop request: %v", looppkg.ErrValidation, err))
		return
	}
	response, err := service.ValidateLoop(c.Request.Context(), c.Param("workspace_id"), c.Param("name"), req)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// DeleteLoop deletes one writable Loop definition.
func (h *BaseHandlers) DeleteLoop(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	if err := service.DeleteLoop(c.Request.Context(), c.Param("workspace_id"), c.Param("name")); err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// RunLoop starts or dry-runs a Loop.
func (h *BaseHandlers) RunLoop(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	var req contract.RunLoopRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: decode run loop request: %v", looppkg.ErrValidation, err))
		return
	}
	dry, err := ParseOptionalBool(c.Query("dry"))
	if err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: dry query: %v", looppkg.ErrValidation, err))
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
		req,
		loopStartKindForTransport(h.transportName()),
		actor,
		dry,
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

// GetLoopConfig returns one no-fork Loop config override.
func (h *BaseHandlers) GetLoopConfig(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	response, err := service.GetLoopConfig(c.Request.Context(), c.Param("workspace_id"), c.Param("name"))
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// PutLoopConfig replaces one no-fork Loop config override.
func (h *BaseHandlers) PutLoopConfig(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	var req contract.PutLoopConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: decode loop config request: %v", looppkg.ErrValidation, err))
		return
	}
	response, err := service.PutLoopConfig(c.Request.Context(), c.Param("workspace_id"), c.Param("name"), req)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// GetLoopAnnotations returns editor node positions for one Loop.
func (h *BaseHandlers) GetLoopAnnotations(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	response, err := service.GetLoopAnnotations(c.Request.Context(), c.Param("workspace_id"), c.Param("name"))
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// PutLoopAnnotations replaces editor node positions for one Loop.
func (h *BaseHandlers) PutLoopAnnotations(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	var req contract.PutLoopAnnotationsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: decode loop annotations request: %v", looppkg.ErrValidation, err))
		return
	}
	response, err := service.PutLoopAnnotations(c.Request.Context(), c.Param("workspace_id"), c.Param("name"), req)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
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
	response, err := service.ListLoopRuns(c.Request.Context(), c.Param("workspace_id"), query)
	if err != nil {
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
	response, err := service.GetLoopRun(c.Request.Context(), c.Param("workspace_id"), c.Param("run_id"))
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// StopLoopRun requests operator stop for one Loop run.
func (h *BaseHandlers) StopLoopRun(c *gin.Context) {
	h.mutateLoopRun(c, func(service LoopService) error {
		workspaceID := c.Param("workspace_id")
		actor, err := h.taskActorContextForWorkspace(c, loopActionStop, workspaceID)
		if err != nil {
			return err
		}
		return service.StopLoopRun(c.Request.Context(), workspaceID, c.Param("run_id"), actor)
	})
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
	if err := mutate(service); err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ParseLoopRunListQuery parses shared Loop run list filters.
func ParseLoopRunListQuery(c *gin.Context) (LoopRunListQuery, error) {
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		return LoopRunListQuery{}, err
	}
	query := LoopRunListQuery{
		LoopName:      strings.TrimSpace(c.Query("loop")),
		Status:        strings.TrimSpace(c.Query("status")),
		Origin:        strings.TrimSpace(c.Query("origin")),
		OriginSession: strings.TrimSpace(c.Query("origin_session")),
		Limit:         limit,
	}
	if raw, present := c.GetQuery("live"); present {
		if strings.TrimSpace(raw) == "" {
			return LoopRunListQuery{}, errors.New("live must be nonempty")
		}
		live, err := ParseOptionalBool(raw)
		if err != nil {
			return LoopRunListQuery{}, err
		}
		query.Live = &live
	}
	return query, nil
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
	h.respondError(c, StatusForLoopError(err), err)
}
