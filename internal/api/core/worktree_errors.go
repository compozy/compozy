package core

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/worktree"
)

func WorktreeErrorCode(err error) string {
	for _, candidate := range worktreeErrorCodes {
		if errors.Is(err, candidate.err) {
			return candidate.code
		}
	}
	return ""
}

func StatusForWorktreeError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, worktree.ErrGitUnavailable),
		errors.Is(err, worktree.ErrGitVersionUnsupported),
		errors.Is(err, worktree.ErrForgeUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, worktree.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, worktree.ErrRefInvalid):
		return http.StatusBadRequest
	case errors.Is(err, worktree.ErrConfigInvalid):
		return http.StatusUnprocessableEntity
	case errors.Is(err, worktree.ErrDeniedByHook):
		return http.StatusForbidden
	case errors.Is(err, worktree.ErrRemovalFailed),
		errors.Is(err, worktree.ErrPerRunMaterialization):
		return http.StatusInternalServerError
	case errors.Is(err, worktree.ErrForge):
		return http.StatusBadGateway
	default:
		if WorktreeErrorCode(err) != "" {
			return http.StatusConflict
		}
		return http.StatusInternalServerError
	}
}

var worktreeErrorCodes = []struct {
	err  error
	code string
}{
	{worktree.ErrGitUnavailable, "worktree_git_unavailable"},
	{worktree.ErrGitVersionUnsupported, "worktree_git_version_unsupported"},
	{worktree.ErrWorkspaceNotGitBacked, "workspace_not_git_backed"},
	{worktree.ErrNameTaken, "worktree_name_taken"},
	{worktree.ErrPathExists, "worktree_path_exists"},
	{worktree.ErrBranchHeld, "branch_held_by_worktree"},
	{worktree.ErrBranchCheckedOutAtRoot, "branch_checked_out_at_root"},
	{worktree.ErrBaseRefNotFound, "base_ref_not_found"},
	{worktree.ErrRepoHasNoCommits, "repo_has_no_commits"},
	{worktree.ErrNotFound, "worktree_not_found"},
	{worktree.ErrNotReady, "worktree_not_ready"},
	{worktree.ErrPending, "worktree_pending"},
	{worktree.ErrMissing, "worktree_missing"},
	{worktree.ErrRefInvalid, "worktree_ref_invalid"},
	{worktree.ErrAdoptionMainCheckout, "adoption_main_checkout"},
	{worktree.ErrAdoptionForeignRepo, "adoption_foreign_repository"},
	{worktree.ErrAdoptionUnreadable, "adoption_unreadable"},
	{worktree.ErrOperationInProgress, "worktree_operation_in_progress"},
	{worktree.ErrSessionActive, "worktree_session_active"},
	{worktree.ErrStatusUnreadable, "worktree_status_unreadable"},
	{worktree.ErrDirtyRequiresForce, "worktree_dirty_requires_force"},
	{worktree.ErrUnpushedRequiresForce, "worktree_unpushed_requires_force"},
	{worktree.ErrSafetyCheckFailed, "worktree_safety_check_failed"},
	{worktree.ErrRemovalFailed, "worktree_removal_failed"},
	{worktree.ErrPerRunMaterialization, "per_run_materialization_failed"},
	{worktree.ErrConfigInvalid, "worktree_config_invalid"},
	{worktree.ErrDeniedByHook, "worktree_denied_by_hook"},
	{worktree.ErrNotPending, "worktree_not_pending"},
	{worktree.ErrForgeUnavailable, "forge_unavailable"},
	{worktree.ErrForge, "forge_error"},
}
