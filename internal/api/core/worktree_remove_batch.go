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

type WorktreeRemovalPreviewer interface {
	PreviewRemoval(
		context.Context, string, worktree.RemovalTarget, bool,
	) (worktree.RemovalRisk, *worktree.RemovalRefusal, error)
}

func ValidateWorktreeRemovalBatch(request contract.RemoveWorktreesRequest) error {
	if len(request.Targets) == 0 || len(request.Targets) > 100 {
		return fmt.Errorf("%w: targets must contain 1 to 100 items", worktree.ErrRefInvalid)
	}
	for _, target := range request.Targets {
		ref, path := strings.TrimSpace(target.Ref), strings.TrimSpace(target.Path)
		if (ref == "") == (path == "") || (path != "" && request.DeleteBranch) || (ref != "" && target.ExpectStale) {
			return fmt.Errorf("%w: each target needs ref or path; only path accepts expect_stale and keeps its branch",
				worktree.ErrRefInvalid)
		}
	}
	return nil
}

func RemoveWorktreeBatch(
	ctx context.Context, service WorktreeService, workspaceID string, request contract.RemoveWorktreesRequest,
	authorize func(*worktree.Worktree) error, formatError func(error) string,
) contract.RemoveWorktreesResponse {
	if formatError == nil {
		formatError = func(err error) string {
			return ErrorPayloadForStatus(StatusForWorktreeError(err), err, false).Error
		}
	}
	response := contract.RemoveWorktreesResponse{
		DryRun: request.DryRun, Results: make([]contract.WorktreeRemovalResult, 0, len(request.Targets)),
	}
	for _, target := range request.Targets {
		result := removeWorktreeBatchItem(ctx, service, workspaceID, target, request, authorize, formatError)
		response.Results = append(response.Results, result)
	}
	return response
}

func removeWorktreeBatchItem(
	ctx context.Context, service WorktreeService, workspaceID string, target contract.WorktreeRemovalTarget,
	request contract.RemoveWorktreesRequest, authorize func(*worktree.Worktree) error, formatError func(error) string,
) contract.WorktreeRemovalResult {
	result := contract.WorktreeRemovalResult{Target: target, State: "removed"}
	if request.DryRun {
		result.State = "ready"
	}
	ref := strings.TrimSpace(target.Ref)
	if ref != "" {
		item, err := service.Resolve(ctx, workspaceID, ref)
		if err == nil && item == nil {
			err = worktree.ErrNotFound
		}
		if err == nil && authorize != nil {
			err = authorize(item)
		}
		if err != nil {
			return failedWorktreeRemoval(result, nil, err, formatError)
		}
		ref, result.WorktreeID = item.ID, item.ID
	}
	var refusalResult *worktree.RemovalRefusal
	var err error
	switch {
	case request.DryRun:
		previewer, ok := service.(WorktreeRemovalPreviewer)
		if !ok {
			return failedWorktreeRemoval(result, nil, errors.New("worktree removal preview unavailable"), formatError)
		}
		var risk worktree.RemovalRisk
		risk, refusalResult, err = previewer.PreviewRemoval(ctx, workspaceID, worktree.RemovalTarget{
			Ref: ref, Path: target.Path, ExpectStale: target.ExpectStale,
		}, request.Force)
		payload := WorktreeRemovalRefusalPayload(worktree.RemovalRefusal{Risk: risk})
		result.Risk = &payload.Risk
	case ref != "":
		refusalResult, err = service.Remove(ctx, workspaceID, ref, request.Force)
	default:
		refusalResult, err = service.RemoveDiscovered(ctx, workspaceID, target.Path, worktree.RemoveDiscoveredOptions{
			Force: request.Force, ExpectStale: target.ExpectStale,
		})
	}
	if err != nil || refusalResult != nil {
		return failedWorktreeRemoval(result, refusalResult, err, formatError)
	}
	if !request.DryRun && request.DeleteBranch {
		result.Branch, err = CleanupBranchOutcome(ctx, service, workspaceID, ref, request.Force)
		if err != nil {
			result.Code = WorktreeErrorCode(err)
			if result.Code == "" {
				result.Code = WorktreeErrorCode(worktree.ErrRemovalFailed)
			}
			result.Error = formatError(err)
		}
	}
	return result
}

func failedWorktreeRemoval(
	result contract.WorktreeRemovalResult,
	refusalResult *worktree.RemovalRefusal,
	err error,
	formatError func(error) string,
) contract.WorktreeRemovalResult {
	result.State = "failed"
	if refusalResult != nil {
		payload := WorktreeRemovalRefusalPayload(*refusalResult)
		result.State, result.Code, result.Risk, result.Error = "refused", payload.Code, &payload.Risk, payload.Error
	}
	if err != nil {
		if result.Error == "" {
			result.Error = formatError(err)
		}
		if result.Code == "" {
			result.Code = WorktreeErrorCode(err)
		}
	}
	if result.Code == "" {
		result.Code = WorktreeErrorCode(worktree.ErrRemovalFailed)
	}
	return result
}

func (h *BaseHandlers) RemoveWorktrees(c *gin.Context) {
	scope, ok := h.worktreeWorkspaceScope(c)
	if !ok {
		return
	}
	var request contract.RemoveWorktreesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("api: decode worktree removal batch: %w", err))
		return
	}
	if err := ValidateWorktreeRemovalBatch(request); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	owners, err := h.profileOwnerIdentities(c.Request.Context())
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	authorize := func(item *worktree.Worktree) error {
		if strings.TrimSpace(c.Query("profile")) != "" || c.Query("all_profiles") != "" ||
			hasAgentCallerIdentityCredentials(agentCallerCredentialsFromRequest(c)) {
			selection, err := h.resolveProfileMutationScope(c)
			if err != nil {
				return err
			}
			if !selection.Matches(item.ProfileID) {
				return worktree.ErrNotFound
			}
		}
		owner, exists := owners[item.ProfileID]
		if !exists || owner.Archived {
			return worktree.ErrNotReady
		}
		return nil
	}
	// Discovered paths have no owner; require the ordinary mutation profile once.
	for _, target := range request.Targets {
		if target.Path != "" {
			if _, err := h.resolveProfileMutationScope(c); err != nil {
				h.respondProfileReadScopeError(c, err)
				return
			}
		}
	}
	formatError := func(err error) string {
		status := StatusForWorktreeError(err)
		h.logServerError(c, status, err)
		return ErrorPayloadForStatus(status, err, h.MaskInternalErrors).Error
	}
	c.JSON(
		http.StatusOK,
		RemoveWorktreeBatch(c.Request.Context(), h.Worktrees, scope.RegistryID, request, authorize, formatError),
	)
}
