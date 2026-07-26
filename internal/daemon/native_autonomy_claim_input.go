package daemon

import (
	"strings"

	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

type autonomyClaimNextInput struct {
	RunID                string   `json:"run_id,omitempty"`
	WorkspaceID          string   `json:"workspace_id,omitempty"`
	RequiredCapabilities []string `json:"required_capabilities,omitempty"`
	PriorityMin          int      `json:"priority_min,omitempty"`
	LeaseSeconds         int64    `json:"lease_seconds,omitempty"`
}

func (i autonomyClaimNextInput) criteria(scope toolspkg.Scope, sessionID string) (taskpkg.ClaimCriteria, error) {
	leaseDuration, err := autonomyLeaseDuration(i.LeaseSeconds)
	if err != nil {
		return taskpkg.ClaimCriteria{}, err
	}
	workspaceID := strings.TrimSpace(i.WorkspaceID)
	if workspaceID == "" {
		workspaceID = strings.TrimSpace(scope.WorkspaceID)
	}
	return taskpkg.ClaimCriteria{
		RunID:            strings.TrimSpace(i.RunID),
		WorkspaceID:      workspaceID,
		ClaimerSessionID: sessionID,
		ClaimedBy: &taskpkg.ActorIdentity{
			Kind: taskpkg.ActorKindAgentSession,
			Ref:  sessionID,
		},
		AgentName:            strings.TrimSpace(scope.AgentName),
		RequiredCapabilities: trimNativeStrings(i.RequiredCapabilities),
		PriorityMin:          i.PriorityMin,
		LeaseDuration:        leaseDuration,
	}, nil
}
