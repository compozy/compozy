package globaldb

import (
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func goalJudgeAttemptFromGenerated(
	row sqlcgen.LoopGoalJudgeAttempt,
	workspaceID loop.WorkspaceID,
) (goal.JudgeAttempt, error) {
	attempt := goal.JudgeAttempt{
		AttemptID: row.AttemptID,
		Key: goal.TurnKey{WorkspaceID: workspaceID, LoopRunID: loop.RunID(row.LoopRunID),
			Generation: int(row.Generation), NodeID: dsl.NodeID(row.NodeID), ItemIndex: int(row.ItemIndex)},
		Turn: int(row.Turn), Status: row.Status, Outcome: row.Outcome.String,
		EvidenceRef: row.EvidenceRef.String, UsageBaseTokens: row.UsageBaseTokens,
		StartedAt: row.StartedAt.UTC(),
	}
	if err := json.Unmarshal([]byte(row.BlockingJson), &attempt.BlockingIssues); err != nil {
		return goal.JudgeAttempt{}, fmt.Errorf("store: decode goal judge blocking issues: %w", err)
	}
	if attempt.BlockingIssues == nil {
		attempt.BlockingIssues = []gate.BlockingIssue{}
	}
	if err := json.Unmarshal([]byte(row.CriteriaJson), &attempt.Criteria); err != nil {
		return goal.JudgeAttempt{}, fmt.Errorf("store: decode goal judge criteria: %w", err)
	}
	if attempt.Criteria == nil {
		attempt.Criteria = []gate.CriterionResult{}
	}
	if err := json.Unmarshal([]byte(row.WarningsJson), &attempt.Warnings); err != nil {
		return goal.JudgeAttempt{}, fmt.Errorf("store: decode goal judge warnings: %w", err)
	}
	if attempt.Warnings == nil {
		attempt.Warnings = []gate.DiagnosticWarning{}
	}
	if row.TokensUsed.Valid {
		attempt.TokensReported = true
		attempt.TokensUsed = row.TokensUsed.Int64
	}
	if row.CompletedAt.Valid {
		value := row.CompletedAt.Time.UTC()
		attempt.CompletedAt = &value
	}
	return attempt, nil
}
