package daemon

import (
	"time"

	"github.com/compozy/compozy/internal/api/contract"
)

type compactLoopRun struct {
	ID                string                       `json:"id"`
	ProfileID         string                       `json:"profile_id"`
	WorkspaceID       string                       `json:"workspace_id"`
	LoopName          string                       `json:"loop_name"`
	Status            contract.LoopRunStatus       `json:"status"`
	Historical        bool                         `json:"historical"`
	CompletionState   contract.LoopCompletionState `json:"completion_state"`
	Generation        int64                        `json:"generation"`
	BestGeneration    *int64                       `json:"best_generation,omitzero"`
	BestScore         *float64                     `json:"best_score,omitzero"`
	ForkedFrom        *contract.LoopForkRef        `json:"forked_from,omitzero"`
	Forks             []contract.LoopForkRef       `json:"forks"`
	CreatedAt         time.Time                    `json:"created_at"`
	StartedAt         time.Time                    `json:"started_at"`
	LastProgressAt    time.Time                    `json:"last_progress_at"`
	CompletedAt       *time.Time                   `json:"completed_at,omitempty"`
	DefinitionVersion int                          `json:"definition_version"`
	DefinitionDigest  string                       `json:"definition_digest,omitempty"`
	ActiveGateID      string                       `json:"active_gate_id,omitempty"`
	BudgetApprovalSeq int                          `json:"budget_approval_seq,omitzero"`
	IterationCap      int                          `json:"iteration_cap"`
	BudgetTokens      int                          `json:"budget_tokens"`
	BudgetWallSec     int                          `json:"budget_wall_sec"`
	BudgetOnExceeded  contract.LoopBudgetExceeded  `json:"budget_on_exceeded"`
	TokensUsed        int64                        `json:"tokens_used"`
	ParentLoopRunID   string                       `json:"parent_loop_run_id,omitempty"`
	PauseRequested    bool                         `json:"pause_requested"`
	Attention         *contract.LoopRunAttention   `json:"attention,omitzero"`
	Progress          contract.LoopRunProgress     `json:"progress"`
}

func projectCompactLoopRun(p *contract.LoopRunPayload) *compactLoopRun {
	if p == nil {
		return nil
	}
	result := &compactLoopRun{
		ID:                p.ID,
		ProfileID:         p.ProfileID,
		WorkspaceID:       p.WorkspaceID,
		LoopName:          p.LoopName,
		Status:            p.Status,
		Historical:        p.Historical,
		CompletionState:   p.CompletionState,
		Generation:        p.Generation,
		BestGeneration:    p.BestGeneration,
		BestScore:         p.BestScore,
		ForkedFrom:        p.ForkedFrom,
		Forks:             p.Forks,
		CreatedAt:         p.CreatedAt,
		StartedAt:         p.StartedAt,
		LastProgressAt:    p.LastProgressAt,
		CompletedAt:       p.CompletedAt,
		DefinitionVersion: p.DefinitionVersion,
		DefinitionDigest:  p.DefinitionDigest,
		ActiveGateID:      p.ActiveGateID,
		BudgetApprovalSeq: p.BudgetApprovalSeq,
		IterationCap:      p.IterationCap,
		BudgetTokens:      p.BudgetTokens,
		BudgetWallSec:     p.BudgetWallSec,
		BudgetOnExceeded:  p.BudgetOnExceeded,
		TokensUsed:        p.TokensUsed,
		ParentLoopRunID:   p.ParentLoopRunID,
		PauseRequested:    p.PauseRequested,
		Attention:         p.Attention,
		Progress:          p.Progress,
	}
	return result
}

type compactLoopStart struct {
	Run    *compactLoopRun           `json:"run,omitzero"`
	DryRun *contract.LoopPlanPayload `json:"dry_run,omitzero"`
	WebURL string                    `json:"web_url,omitempty"`
}

func projectCompactLoopStart(p *contract.RunLoopResponse) *compactLoopStart {
	if p == nil {
		return nil
	}
	result := &compactLoopStart{
		DryRun: p.DryRun,
		WebURL: p.WebURL,
	}
	result.Run = projectCompactLoopRun(p.Run)
	return result
}

func viewLoopStart(p contract.RunLoopResponse) any { return projectCompactLoopStart(&p) }

type compactLoopFork struct {
	Run      compactLoopRun `json:"run"`
	Replayed bool           `json:"replayed,omitzero"`
}

func projectCompactLoopFork(p *contract.ForkLoopResponse) *compactLoopFork {
	if p == nil {
		return nil
	}
	result := &compactLoopFork{
		Replayed: p.Replayed,
	}
	result.Run = *projectCompactLoopRun(&p.Run)
	return result
}

func viewLoopFork(p *contract.ForkLoopResponse) any { return projectCompactLoopFork(p) }

type compactLoopRuns struct {
	Runs       []compactLoopRun                  `json:"runs"`
	Aggregates contract.LoopRunsAggregatePayload `json:"aggregates"`
	NextCursor string                            `json:"next_cursor,omitempty"`
}

func projectCompactLoopRuns(p *contract.LoopRunsResponse) *compactLoopRuns {
	if p == nil {
		return nil
	}
	result := &compactLoopRuns{
		Aggregates: p.Aggregates,
		NextCursor: p.NextCursor,
	}
	if p.Runs != nil {
		result.Runs = make([]compactLoopRun, len(p.Runs))
		for i := range p.Runs {
			result.Runs[i] = *projectCompactLoopRun(&p.Runs[i])
		}
	}
	return result
}

func viewLoopRuns(p contract.LoopRunsResponse) any { return projectCompactLoopRuns(&p) }
