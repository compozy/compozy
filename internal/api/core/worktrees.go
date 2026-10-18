package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/worktree"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) ListWorktrees(c *gin.Context) {
	scope, ok := h.worktreeWorkspaceScope(c)
	if !ok {
		return
	}
	refresh, ok := h.worktreeBoolQuery(c, "refresh")
	if !ok {
		return
	}
	listing, err := h.Worktrees.ListDetails(c.Request.Context(), scope.RegistryID, refresh)
	if err != nil {
		h.respondError(c, StatusForWorktreeError(err), err)
		return
	}
	if err := h.enrichWorktreeListingOwners(c.Request.Context(), listing); err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, WorktreesPayloadFromListing(listing))
}

func (h *BaseHandlers) CreateWorktree(c *gin.Context) {
	scope, ok := h.worktreeWorkspaceScope(c)
	if !ok {
		return
	}
	var request contract.CreateWorktreeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("api: decode create worktree request: %w", err))
		return
	}
	mutationScope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	item, err := h.Worktrees.CreateAccepted(c.Request.Context(), scope.RegistryID, worktree.CreateOptions{
		ProfileID: mutationScope.ProfileID,
		Name:      strings.TrimSpace(request.Name), Branch: strings.TrimSpace(request.Branch),
		BaseRef: strings.TrimSpace(request.BaseRef), ExistingBranch: strings.TrimSpace(request.ExistingBranch),
		Path: strings.TrimSpace(request.Path), Origin: worktree.OriginManual,
	})
	if err != nil {
		h.respondError(c, StatusForWorktreeError(err), err)
		return
	}
	if err := h.enrichWorktreeOwner(c.Request.Context(), item); err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusAccepted, contract.WorktreeResponse{
		Worktree: WorktreePayloadFromInspection(worktree.Inspection{
			Worktree: *item, AgentActivity: worktree.AgentActivityIdle,
		}),
	})
}

func (h *BaseHandlers) AdoptWorktree(c *gin.Context) {
	scope, ok := h.worktreeWorkspaceScope(c)
	if !ok {
		return
	}
	var request contract.AdoptWorktreeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("api: decode adopt worktree request: %w", err))
		return
	}
	mutationScope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	item, err := h.Worktrees.Adopt(
		c.Request.Context(),
		mutationScope.ProfileID,
		scope.RegistryID,
		strings.TrimSpace(request.Path),
	)
	if err != nil {
		h.respondError(c, StatusForWorktreeError(err), err)
		return
	}
	if err := h.enrichWorktreeOwner(c.Request.Context(), item); err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, contract.WorktreeResponse{
		Worktree: WorktreePayloadFromInspection(worktree.Inspection{
			Worktree: *item, AgentActivity: worktree.AgentActivityIdle,
		}),
	})
}

func (h *BaseHandlers) InspectWorktree(c *gin.Context) {
	scope, id, ok := h.worktreeRoute(c)
	if !ok {
		return
	}
	inspection, err := h.Worktrees.Inspect(c.Request.Context(), scope.RegistryID, id)
	if err != nil {
		h.respondError(c, StatusForWorktreeError(err), err)
		return
	}
	if err := h.enrichWorktreeOwner(c.Request.Context(), &inspection.Worktree); err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, WorktreeInspectionPayload(*inspection))
}

func (h *BaseHandlers) GetWorktreeStatus(c *gin.Context) {
	scope, id, ok := h.worktreeRoute(c)
	if !ok {
		return
	}
	refresh, ok := h.worktreeBoolQuery(c, "refresh")
	if !ok {
		return
	}
	refreshForge, ok := h.worktreeBoolQuery(c, "forge")
	if !ok {
		return
	}
	details, err := h.Worktrees.StatusDetails(
		c.Request.Context(), scope.RegistryID, id, refresh, refreshForge,
	)
	if err != nil {
		h.respondError(c, StatusForWorktreeError(err), err)
		return
	}
	status := WorktreeStatusPayloadFromStatus(details.Status)
	if status == nil {
		status = &contract.WorktreeStatusPayload{}
	}
	c.JSON(http.StatusOK, contract.WorktreeStatusResponse{
		WorktreeID: details.WorktreeID,
		Status:     *status,
		Forge:      WorktreeForgePayloadFromStatus(details.ForgeStatus),
	})
}

func (h *BaseHandlers) GetWorktreeExitPlan(c *gin.Context) {
	scope, id, ok := h.worktreeRoute(c)
	if !ok {
		return
	}
	var plan *worktree.ExitPlan
	var err error
	if paths := c.QueryArray("include"); len(paths) > 0 {
		scoped, supports := h.Worktrees.(interface {
			ExitPlanForPaths(context.Context, string, string, []string) (*worktree.ExitPlan, error)
		})
		if !supports {
			h.respondError(c, http.StatusBadRequest, errors.New("api: scoped exit plan unavailable"))
			return
		}
		plan, err = scoped.ExitPlanForPaths(c.Request.Context(), scope.RegistryID, id, paths)
	} else {
		plan, err = h.Worktrees.ExitPlan(c.Request.Context(), scope.RegistryID, id)
	}
	if err != nil {
		h.respondError(c, StatusForWorktreeError(err), err)
		return
	}
	c.JSON(http.StatusOK, WorktreeExitPlanPayload(plan))
}

func (h *BaseHandlers) RunWorktreeExitAction(c *gin.Context) {
	scope, id, ok := h.worktreeRoute(c)
	if !ok {
		return
	}
	var request contract.RunWorktreeExitActionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("api: decode worktree exit action: %w", err))
		return
	}
	if !validWorktreeExitAction(request.Action) {
		h.respondError(c, http.StatusBadRequest, errors.New("api: invalid worktree exit action"))
		return
	}
	exitRequest := worktree.ExitActionRequest{
		Action:        worktree.ExitAction(request.Action),
		Message:       request.Message,
		Title:         request.Title,
		Body:          request.Body,
		Draft:         request.Draft,
		Base:          request.Base,
		IncludePaths:  request.IncludePaths,
		ExpectedScope: request.ExpectedScope,
	}
	exitRequest.DeliveryID, exitRequest.ExpectedHead = request.DeliveryID, request.ExpectedHead
	var opID string
	var err error
	if request.Action == "deliver" {
		caller, ok := h.requireAgentCaller(c, "worktree.deliver")
		if !ok {
			return
		}
		managed, ok := h.Worktrees.(interface {
			SubmitManagedDelivery(context.Context, string, string, string, worktree.ExitActionRequest) (string, error)
		})
		if !ok {
			h.respondError(c, http.StatusServiceUnavailable, worktree.ErrForgeUnavailable)
			return
		}
		if caller.Session.WorkspaceID != scope.RegistryID {
			h.respondError(c, http.StatusForbidden, errors.New("api: delivery caller belongs to another workspace"))
			return
		}
		opID, err = managed.SubmitManagedDelivery(
			c.Request.Context(),
			scope.RegistryID,
			id,
			caller.Session.ID,
			exitRequest,
		)
	} else {
		opID, err = h.Worktrees.RunExitAction(c.Request.Context(), scope.RegistryID, id, exitRequest)
	}
	if err != nil {
		h.respondError(c, StatusForWorktreeError(err), err)
		return
	}
	c.JSON(http.StatusAccepted, contract.WorktreeExitOperationResponse{OperationID: opID})
}

func (h *BaseHandlers) CancelWorktreeExitAction(c *gin.Context) {
	scope, id, ok := h.worktreeRoute(c)
	if !ok {
		return
	}
	var request contract.CancelWorktreeExitActionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("api: decode worktree exit cancel: %w", err))
		return
	}
	opID := strings.TrimSpace(request.OperationID)
	if opID == "" {
		h.respondError(c, http.StatusBadRequest, errors.New("api: op_id is required"))
		return
	}
	if err := h.Worktrees.CancelExitAction(c.Request.Context(), scope.RegistryID, id, opID); err != nil {
		h.respondError(c, StatusForWorktreeError(err), err)
		return
	}
	c.Status(http.StatusNoContent)
}

func validWorktreeExitAction(action contract.WorktreeExitAction) bool {
	switch worktree.ExitAction(action) {
	case worktree.ExitActionDeliver,
		worktree.ExitActionCommit,
		worktree.ExitActionCommitPush,
		worktree.ExitActionPush,
		worktree.ExitActionOpenPR:
		return true
	default:
		return false
	}
}

func (h *BaseHandlers) CancelWorktreeCreate(c *gin.Context) {
	scope, id, ok := h.worktreeRoute(c)
	if !ok {
		return
	}
	if err := h.Worktrees.CancelCreate(c.Request.Context(), scope.RegistryID, id); err != nil {
		h.respondError(c, StatusForWorktreeError(err), err)
		return
	}
	c.Status(http.StatusNoContent)
}

// RemoveWorktree authorizes the saved target before delegating checkout removal.
// Lifecycle refusals retain their structured 409 payload, including force risks.
func (h *BaseHandlers) RemoveWorktree(c *gin.Context) {
	scope, id, ok := h.worktreeRoute(c)
	if !ok {
		return
	}
	id, ok = h.worktreeMutationTarget(c, scope.RegistryID, id)
	if !ok {
		return
	}
	force, ok := h.worktreeBoolQuery(c, "force")
	if !ok {
		return
	}
	refusal, err := h.Worktrees.Remove(c.Request.Context(), scope.RegistryID, id, force)
	if refusal != nil {
		c.JSON(http.StatusConflict, WorktreeRemovalRefusalPayload(*refusal))
		return
	}
	if err != nil {
		h.respondError(c, StatusForWorktreeError(err), err)
		return
	}
	c.Status(http.StatusNoContent)
}

// DismissWorktree authorizes metadata cleanup without deleting files or history.
// The lifecycle service owns repeated-request and concurrent-state handling.
func (h *BaseHandlers) DismissWorktree(c *gin.Context) {
	scope, id, ok := h.worktreeRoute(c)
	if !ok {
		return
	}
	id, ok = h.worktreeMutationTarget(c, scope.RegistryID, id)
	if !ok {
		return
	}
	if err := h.Worktrees.Dismiss(c.Request.Context(), scope.RegistryID, id); err != nil {
		h.respondError(c, StatusForWorktreeError(err), err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *BaseHandlers) worktreeWorkspaceScope(c *gin.Context) (workspaceScope, bool) {
	if h.Worktrees == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: worktree service is required"))
		return workspaceScope{}, false
	}
	return h.resolveWorkspaceScope(c)
}

func (h *BaseHandlers) worktreeRoute(c *gin.Context) (workspaceScope, string, bool) {
	scope, ok := h.worktreeWorkspaceScope(c)
	if !ok {
		return workspaceScope{}, "", false
	}
	id := strings.TrimSpace(c.Param("worktree_id"))
	if id == "" {
		h.respondError(c, http.StatusBadRequest, errors.New("api: worktree_id path is required"))
		return workspaceScope{}, "", false
	}
	return scope, id, true
}

func (h *BaseHandlers) worktreeBoolQuery(c *gin.Context, key string) (bool, bool) {
	raw, exists := c.GetQuery(key)
	if !exists {
		return false, true
	}
	value, err := ParseOptionalBool(raw)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("api: invalid %s: %w", key, err))
		return false, false
	}
	return value, true
}
