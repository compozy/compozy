package core

import (
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/worktree"
)

func WorktreePayloadFromInspection(inspection worktree.Inspection) contract.WorktreePayload {
	item := inspection.Worktree
	payload := contract.WorktreePayload{
		ID: item.ID, WorkspaceID: item.WorkspaceID, Name: item.Name, Branch: item.Branch,
		Path: item.Path, State: string(item.State), PendingPhase: string(item.PendingPhase),
		Origin: string(item.Origin), SetupState: string(item.SetupState), SetupError: item.SetupError,
		BaseRef: item.BaseRef, CreatedBranch: item.CreatedBranch, RunNamespace: item.RunNamespace,
		CreatedHead: item.CreatedHead, RunID: item.RunID, AgentActivity: string(inspection.AgentActivity),
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
	if inspection.Status != nil {
		payload.Ahead = inspection.Status.Ahead
		if payload.Ahead == nil {
			payload.Ahead = inspection.Status.AheadOfBase
		}
		payload.Behind = inspection.Status.Behind
		if inspection.Status.DirtyFiles != nil {
			dirty := *inspection.Status.DirtyFiles > 0
			payload.Dirty = &dirty
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
		Code: refusal.Code,
		Risk: contract.WorktreeRemovalRiskPayload{
			ChangedFiles: refusal.Risk.ChangedFiles, Insertions: refusal.Risk.Insertions,
			Deletions: refusal.Risk.Deletions, UnpushedCommits: refusal.Risk.UnpushedCommits,
			ExistsOnRemote: refusal.Risk.ExistsOnRemote,
		},
		Downgrade: refusal.Downgrade,
	}
}
