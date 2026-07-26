package globaldb

import (
	"encoding/json"
	"fmt"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/gate"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
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
