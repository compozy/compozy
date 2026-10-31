package core

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/worktree"
)

func WorktreePayloadFromInspection(inspection worktree.Inspection) contract.WorktreePayload {
	item := inspection.Worktree
	payload := contract.WorktreePayload{
		ID: item.ID, ProfileID: item.ProfileID, ProfileName: item.ProfileName,
		ProfileColor: item.ProfileColor, ProfileIcon: item.ProfileIcon,
		ProfileEmoji: item.ProfileEmoji, ProfileArchived: item.ProfileArchived,
		WorkspaceID: item.WorkspaceID, Name: item.Name, Branch: item.Branch,
		Path: item.Path, State: string(item.State), PendingPhase: string(item.PendingPhase),
		Origin: string(item.Origin), SetupState: string(item.SetupState), SetupError: item.SetupError,
		CreateError: item.CreateFailure(), BaseRef: item.BaseRef, CreatedBranch: item.CreatedBranch,
		RunNamespace: item.RunNamespace, CreatedHead: item.CreatedHead, RunID: item.RunID,
		AgentActivity: string(inspection.AgentActivity), CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
	if inspection.Status != nil {
		payload.Ahead = inspection.Status.Ahead
		if payload.Ahead == nil {
			payload.Ahead = inspection.Status.AheadOfBase
		}
		payload.Behind = inspection.Status.Behind
		if inspection.Status.DirtyFiles != nil {
			payload.Dirty = new(*inspection.Status.DirtyFiles > 0)
		}
	}
	return payload
}

func WorktreeStatusPayloadFromStatus(status *worktree.Status) *contract.WorktreeStatusPayload {
	if status == nil {
		return nil
	}
	return &contract.WorktreeStatusPayload{
		Branch: status.Branch, Detached: status.Detached, HeadSHA: status.HeadSHA,
		DirtyFiles: status.DirtyFiles, Insertions: status.Insertions, Deletions: status.Deletions,
		HasUpstream: status.HasUpstream, Ahead: status.Ahead, AheadOfBase: status.AheadOfBase,
		Behind: status.Behind, ReadError: status.ReadError, RefreshedAt: status.RefreshedAt,
	}
}

func WorktreeForgePayloadFromStatus(status *worktree.ForgeStatus) *contract.WorktreeForgeStatusPayload {
	if status == nil {
		return nil
	}
	return &contract.WorktreeForgeStatusPayload{
		Provider: status.Provider, PRNumber: status.PRNumber, PRState: status.PRState,
		PRURL: status.PRURL, Merged: status.Merged, FetchedAt: status.FetchedAt,
	}
}

func WorktreeInspectionPayload(
	inspection worktree.Inspection,
) contract.WorktreeInspectionResponse {
	return contract.WorktreeInspectionResponse{
		Worktree: WorktreePayloadFromInspection(inspection),
		Status:   WorktreeStatusPayloadFromStatus(inspection.Status),
		Forge:    WorktreeForgePayloadFromStatus(inspection.ForgeStatus),
		Repo: contract.WorktreeRepoPayload{
			GitBacked: inspection.Repo.GitBacked, GitAvailable: inspection.Repo.GitAvailable,
			Diagnostic: inspection.Repo.Diagnostic,
		},
		Bindings: contract.WorktreeBindingsPayload{AgentActivity: string(inspection.AgentActivity)},
	}
}

func WorktreesPayloadFromListing(listing *worktree.DetailedListing) contract.WorktreesResponse {
	payload := contract.WorktreesResponse{
		Worktrees:  make([]contract.WorktreePayload, 0),
		Discovered: make([]contract.DiscoveredWorktreePayload, 0),
	}
	if listing == nil {
		return payload
	}
	payload.Worktrees = make([]contract.WorktreePayload, 0, len(listing.Worktrees))
	for _, inspection := range listing.Worktrees {
		payload.Worktrees = append(payload.Worktrees, WorktreePayloadFromInspection(inspection))
	}
	payload.Discovered = make([]contract.DiscoveredWorktreePayload, 0, len(listing.Discovered))
	for _, item := range listing.Discovered {
		payload.Discovered = append(payload.Discovered, contract.DiscoveredWorktreePayload{
			Name: item.Name, Branch: item.Branch, Path: item.Path, Detached: item.Detached,
			Stale: item.Stale, Selectable: item.Selectable, Unavailable: item.Unavailable, Reason: item.Reason,
		})
	}
	payload.Repo = contract.WorktreeRepoPayload{
		GitBacked:    listing.Repo.GitBacked,
		GitAvailable: listing.Repo.GitAvailable,
		Diagnostic:   listing.Repo.Diagnostic,
	}
	return payload
}

func WorktreeRemovalRefusalPayload(
	refusal worktree.RemovalRefusal,
) contract.WorktreeRemovalRefusalPayload {
	return contract.WorktreeRemovalRefusalPayload{
		Code:  refusal.Code,
		Error: worktreeRemovalRefusalMessage(refusal.Code),
		Risk: contract.WorktreeRemovalRiskPayload{
			ChangedFiles: refusal.Risk.ChangedFiles, Insertions: refusal.Risk.Insertions,
			Deletions: refusal.Risk.Deletions, UnpushedCommits: refusal.Risk.UnpushedCommits,
			ExistsOnRemote: refusal.Risk.ExistsOnRemote, LeftoverFiles: refusal.Risk.LeftoverFiles,
		},
		Downgrade: refusal.Downgrade,
	}
}

// worktreeRemovalRefusalMessage names why removal stopped and what force does.
func worktreeRemovalRefusalMessage(code string) string {
	switch code {
	case worktree.ErrDirtyRequiresForce.Error():
		return "The worktree has uncommitted changes. Force removal discards them."
	case worktree.ErrUnpushedRequiresForce.Error():
		return "The worktree has commits that exist nowhere else. Force removal discards them."
	case worktree.ErrCheckoutOrphaned.Error():
		return "The checkout's Git metadata (.git) is missing; the directory is left over from an " +
			"interrupted removal. Force removal deletes the leftover files and clears Git's entry."
	default:
		return ""
	}
}

// WorktreeBranchOutcomePayload maps an opt-in branch deletion outcome to its wire shape.
func WorktreeBranchOutcomePayload(outcome worktree.BranchOutcome) contract.WorktreeBranchOutcomePayload {
	return contract.WorktreeBranchOutcomePayload{
		Name: outcome.Name, Deleted: outcome.Deleted, Reason: outcome.Reason, Detail: outcome.Detail,
	}
}

// CleanupBranchOutcome runs the opted-in branch step after a successful removal
// or dismissal. Only precondition failures stay errors; any other failure is a
// kept-branch outcome, so callers never mistake a completed cleanup for a failed one.
func CleanupBranchOutcome(
	ctx context.Context,
	worktrees WorktreeService,
	workspaceID, id string,
	force bool,
) (*contract.WorktreeBranchOutcomePayload, error) {
	outcome, err := worktrees.DeleteLocalBranch(ctx, workspaceID, id, force)
	if errors.Is(err, worktree.ErrNotReady) || errors.Is(err, worktree.ErrNotFound) {
		return nil, err
	}
	if err != nil {
		outcome.Deleted = false
		outcome.Reason = worktree.BranchKeptDeleteFailed
		outcome.Detail = diagnostics.RedactAndBound(err.Error(), 2048)
	}
	payload := WorktreeBranchOutcomePayload(outcome)
	return &payload, nil
}
