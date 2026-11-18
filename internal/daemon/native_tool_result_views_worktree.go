package daemon

import (
	"github.com/compozy/compozy/internal/api/contract"
)

type compactWorktree struct {
	ProfileID       string `json:"profile_id"`
	ProfileArchived bool   `json:"profile_archived"`

	ID            string `json:"id"`
	WorkspaceID   string `json:"workspace_id"`
	Name          string `json:"name"`
	Branch        string `json:"branch"`
	Path          string `json:"path"`
	State         string `json:"state"`
	PendingPhase  string `json:"pending_phase,omitempty"`
	Origin        string `json:"origin"`
	SetupState    string `json:"setup_state"`
	SetupError    string `json:"setup_error,omitempty"`
	CreateError   string `json:"create_error,omitempty"`
	CreatedBranch bool   `json:"created_branch"`
	RunID         string `json:"run_id,omitempty"`
	Dirty         *bool  `json:"dirty"`
	Ahead         *int   `json:"ahead"`
	Behind        *int   `json:"behind"`
	AgentActivity string `json:"agent_activity"`
}

func projectCompactWorktree(p contract.WorktreePayload) compactWorktree {
	return compactWorktree{
		ProfileID:       p.ProfileID,
		ProfileArchived: p.ProfileArchived,
		ID:              p.ID,
		WorkspaceID:     p.WorkspaceID,
		Name:            p.Name,
		Branch:          p.Branch,
		Path:            p.Path,
		State:           p.State,
		PendingPhase:    p.PendingPhase,
		Origin:          p.Origin,
		SetupState:      p.SetupState,
		SetupError:      p.SetupError,
		CreateError:     p.CreateError,
		CreatedBranch:   p.CreatedBranch,
		RunID:           p.RunID,
		Dirty:           p.Dirty,
		Ahead:           p.Ahead,
		Behind:          p.Behind,
		AgentActivity:   p.AgentActivity,
	}
}

func compactWorktrees(p contract.WorktreesResponse) any {
	var items []compactWorktree
	if p.Worktrees != nil {
		items = make([]compactWorktree, len(p.Worktrees))
		for i := range p.Worktrees {
			items[i] = projectCompactWorktree(p.Worktrees[i])
		}
	}
	return struct {
		Worktrees  []compactWorktree                    `json:"worktrees"`
		Discovered []contract.DiscoveredWorktreePayload `json:"discovered"`
		Repo       contract.WorktreeRepoPayload         `json:"repo"`
	}{items, p.Discovered, p.Repo}
}
