package core

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/gin-gonic/gin"
)

const (
	loopActionCancel      = "loop_cancel"
	loopActionKill        = "loop_kill"
	loopActionNodePause   = "loop_node_pause"
	loopActionNodeResume  = "loop_node_resume"
	loopActionNodeCancel  = "loop_node_cancel"
	loopActionNodeKill    = "loop_node_kill"
	loopActionNodeList    = "loop_node_list"
	loopActionNodeRequeue = "loop_node_requeue"
)

// CancelLoopRun requests cooperative cancellation for one run.
func (h *BaseHandlers) CancelLoopRun(c *gin.Context) {
	h.mutateLoopLifecycle(c, loopActionCancel, func(service LoopService) (contract.LoopMutationResponse, error) {
		actor, err := h.taskActorContextForWorkspace(c, loopActionCancel, c.Param("workspace_id"))
		if err != nil {
			return contract.LoopMutationResponse{}, err
		}
		return service.CancelLoopRun(c.Request.Context(), c.Param("workspace_id"), c.Param("run_id"), actor)
	})
}

// KillLoopRun immediately fences and terminalizes one run.
func (h *BaseHandlers) KillLoopRun(c *gin.Context) {
	h.mutateLoopLifecycle(c, loopActionKill, func(service LoopService) (contract.LoopMutationResponse, error) {
		actor, err := h.taskActorContextForWorkspace(c, loopActionKill, c.Param("workspace_id"))
		if err != nil {
			return contract.LoopMutationResponse{}, err
		}
		return service.KillLoopRun(c.Request.Context(), c.Param("workspace_id"), c.Param("run_id"), actor)
	})
}

// PauseLoopNode parks one authored node.
func (h *BaseHandlers) PauseLoopNode(c *gin.Context) {
	var req contract.LoopNodePauseRequest
	if err := decodeOptionalJSON(c, &req); err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: decode node pause request: %v", looppkg.ErrValidation, err))
		return
	}
	h.mutateLoopNode(c, loopActionNodePause, func(
		service LoopService,
	) (contract.LoopMutationResponse, error) {
		actor, err := h.taskActorContextForWorkspace(c, loopActionNodePause, c.Param("workspace_id"))
		if err != nil {
			return contract.LoopMutationResponse{}, err
		}
		return service.PauseLoopNode(
			c.Request.Context(), c.Param("workspace_id"), c.Param("run_id"), c.Param("node_id"), req, actor,
		)
	})
}

// ResumeLoopNode releases one node pause or admits a manual wait payload.
func (h *BaseHandlers) ResumeLoopNode(c *gin.Context) {
	var req contract.LoopNodeResumeRequest
	if err := decodeOptionalJSON(c, &req); err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: decode node resume request: %v", looppkg.ErrValidation, err))
		return
	}
	h.mutateLoopNode(c, loopActionNodeResume, func(
		service LoopService,
	) (contract.LoopMutationResponse, error) {
		actor, err := h.taskActorContextForWorkspace(c, loopActionNodeResume, c.Param("workspace_id"))
		if err != nil {
			return contract.LoopMutationResponse{}, err
		}
		return service.ResumeLoopNode(
			c.Request.Context(), c.Param("workspace_id"), c.Param("run_id"), c.Param("node_id"), req, actor,
		)
	})
}

// CancelLoopNode requests cooperative cancellation for one authored node.
func (h *BaseHandlers) CancelLoopNode(c *gin.Context) {
	h.mutateLoopNodeRequest(c, loopActionNodeCancel, func(
		service LoopService,
		req contract.LoopNodeMutationRequest,
	) (contract.LoopMutationResponse, error) {
		actor, err := h.taskActorContextForWorkspace(c, loopActionNodeCancel, c.Param("workspace_id"))
		if err != nil {
			return contract.LoopMutationResponse{}, err
		}
		return service.CancelLoopNode(
			c.Request.Context(), c.Param("workspace_id"), c.Param("run_id"), c.Param("node_id"), req, actor,
		)
	})
}

// KillLoopNode immediately fences one authored node.
func (h *BaseHandlers) KillLoopNode(c *gin.Context) {
	h.mutateLoopNodeRequest(c, loopActionNodeKill, func(
		service LoopService,
		req contract.LoopNodeMutationRequest,
	) (contract.LoopMutationResponse, error) {
		actor, err := h.taskActorContextForWorkspace(c, loopActionNodeKill, c.Param("workspace_id"))
		if err != nil {
			return contract.LoopMutationResponse{}, err
		}
		return service.KillLoopNode(
			c.Request.Context(), c.Param("workspace_id"), c.Param("run_id"), c.Param("node_id"), req, actor,
		)
	})
}

// RequeueLoopNode clears one quarantine and reserves the next generation.
func (h *BaseHandlers) RequeueLoopNode(c *gin.Context) {
	h.mutateLoopNodeRequest(c, loopActionNodeRequeue, func(
		service LoopService,
		req contract.LoopNodeMutationRequest,
	) (contract.LoopMutationResponse, error) {
		actor, err := h.taskActorContextForWorkspace(c, loopActionNodeRequeue, c.Param("workspace_id"))
		if err != nil {
			return contract.LoopMutationResponse{}, err
		}
		return service.RequeueLoopNode(
			c.Request.Context(), c.Param("workspace_id"), c.Param("run_id"), c.Param("node_id"), req, actor,
		)
	})
}

// ListLoopNodes returns one stable workspace node-state page.
func (h *BaseHandlers) ListLoopNodes(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	if _, err := h.taskActorContextForWorkspace(c, loopActionNodeList, c.Param("workspace_id")); err != nil {
		h.respondLoopError(c, err)
		return
	}
	query, err := ParseLoopNodeListQuery(c)
	if err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: loop node inventory query: %v", looppkg.ErrValidation, err))
		return
	}
	response, err := service.ListLoopNodes(c.Request.Context(), c.Param("workspace_id"), query)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// ParseLoopNodeListQuery parses shared HTTP and UDS inventory filters.
func ParseLoopNodeListQuery(c *gin.Context) (LoopNodeListQuery, error) {
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		return LoopNodeListQuery{}, err
	}
	return LoopNodeListQuery{
		State:    strings.TrimSpace(c.Query("state")),
		LoopName: strings.TrimSpace(c.Query("loop")),
		RunID:    strings.TrimSpace(c.Query("run_id")),
		Cursor:   strings.TrimSpace(c.Query("cursor")),
		Limit:    limit,
	}, nil
}

func (h *BaseHandlers) mutateLoopNodeRequest(
	c *gin.Context,
	action string,
	mutate func(LoopService, contract.LoopNodeMutationRequest) (contract.LoopMutationResponse, error),
) {
	var req contract.LoopNodeMutationRequest
	if err := decodeOptionalJSON(c, &req); err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: decode %s request: %v", looppkg.ErrValidation, action, err))
		return
	}
	h.mutateLoopNode(c, action, func(service LoopService) (contract.LoopMutationResponse, error) {
		return mutate(service, req)
	})
}

func (h *BaseHandlers) mutateLoopNode(
	c *gin.Context,
	action string,
	mutate func(LoopService) (contract.LoopMutationResponse, error),
) {
	h.mutateLoopLifecycle(c, action, mutate)
}

func (h *BaseHandlers) mutateLoopLifecycle(
	c *gin.Context,
	_ string,
	mutate func(LoopService) (contract.LoopMutationResponse, error),
) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	response, err := mutate(service)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}
