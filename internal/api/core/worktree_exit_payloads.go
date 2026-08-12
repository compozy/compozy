package core

import (
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/worktree"
)

func WorktreeExitPlanPayload(plan *worktree.ExitPlan) contract.WorktreeExitPlanResponse {
	if plan == nil {
		return contract.WorktreeExitPlanResponse{Actions: []contract.WorktreeExitActionPlan{}}
	}
	payload := contract.WorktreeExitPlanResponse{
		WorktreeID: plan.WorktreeID, Primary: contract.WorktreeExitAction(plan.Primary),
		Actions:          make([]contract.WorktreeExitActionPlan, 0, len(plan.Actions)),
		GlobalPauseCause: plan.GlobalPauseCause,
		CommitScope: contract.WorktreeExitCommitScope{
			ChangedFiles: plan.CommitScope.ChangedFiles, Insertions: plan.CommitScope.Insertions,
			Deletions: plan.CommitScope.Deletions,
			UntrackedFiles: append(make([]string, 0, len(plan.CommitScope.UntrackedFiles)),
				plan.CommitScope.UntrackedFiles...),
			UntrackedTotal: plan.CommitScope.UntrackedTotal, UntrackedTruncated: plan.CommitScope.UntrackedTruncated,
		},
		BrowserURL:  plan.BrowserURL,
		ForgeStatus: WorktreeForgePayloadFromStatus(plan.ForgeStatus),
		Cleanup: contract.WorktreeExitCleanupEvidence{
			ForgeState: plan.Cleanup.ForgeState, Safe: plan.Cleanup.Safe, Source: plan.Cleanup.Source,
			Summary: plan.Cleanup.Summary, Blocker: plan.Cleanup.Blocker, Downgraded: plan.Cleanup.Downgraded,
		},
		Base: plan.Base,
	}
	for _, action := range plan.Actions {
		payload.Actions = append(payload.Actions, contract.WorktreeExitActionPlan{
			Action: contract.WorktreeExitAction(action.Action), Label: action.Label,
			Enabled: action.Enabled, BlockedReason: action.BlockedReason, Publish: action.Publish,
			URL: action.URL, PRNumber: action.PRNumber,
		})
	}
	if plan.Forge != nil {
		payload.Forge = &contract.WorktreeExitForgeCapabilities{
			Provider: plan.Forge.Provider, RequestNoun: plan.Forge.RequestNoun,
			OpenActionLabel: plan.Forge.OpenActionLabel, ViewActionLabel: plan.Forge.ViewActionLabel,
			SupportsDraft: plan.Forge.SupportsDraft, CompareURLTemplate: plan.Forge.CompareURLTemplate,
			CredentialSource: plan.Forge.CredentialSource, DefaultBranch: plan.Forge.DefaultBranch,
			TemplatePaths: append([]string(nil), plan.Forge.TemplatePaths...),
		}
	}
	return payload
}
