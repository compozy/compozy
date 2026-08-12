package cli

import "github.com/spf13/cobra"

func worktreeExitPlanBundle(plan WorktreeExitPlanRecord) outputBundle {
	return outputBundle{
		jsonValue: plan,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, plan)
		},
		human: func() (string, error) {
			return renderHumanSection("Worktree Exit", []keyValue{
				{Label: "Worktree", Value: stringOrDash(plan.WorktreeID)},
				{Label: "Primary", Value: stringOrDash(string(plan.Primary))},
				{Label: "Base", Value: stringOrDash(plan.Base)},
				{Label: "Pause", Value: stringOrDash(plan.GlobalPauseCause)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject("worktree_exit",
				[]string{"worktree_id", "primary", "base", "global_pause_cause"},
				[]string{plan.WorktreeID, string(plan.Primary), plan.Base, plan.GlobalPauseCause},
			), nil
		},
	}
}

func worktreeExitOperationBundle(operation WorktreeExitOperationRecord) outputBundle {
	return outputBundle{
		jsonValue: operation,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, operation)
		},
		human: func() (string, error) {
			return renderHumanSection("Worktree Exit", []keyValue{
				{Label: "Operation", Value: stringOrDash(operation.OperationID)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject("worktree_exit_operation", []string{"op_id"}, []string{operation.OperationID}), nil
		},
	}
}

func worktreeExitCancelBundle(worktreeID, operationID string) outputBundle {
	value := struct {
		WorktreeID  string `json:"worktree_id"`
		OperationID string `json:"op_id"`
		State       string `json:"state"`
	}{WorktreeID: worktreeID, OperationID: operationID, State: "cancel_requested"}
	return outputBundle{
		jsonValue: value,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, value)
		},
		human: func() (string, error) {
			return renderHumanSection("Worktree Exit", []keyValue{
				{Label: "Worktree", Value: stringOrDash(worktreeID)},
				{Label: "Operation", Value: stringOrDash(operationID)},
				{Label: "State", Value: "cancel_requested"},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject("worktree_exit_operation",
				[]string{"worktree_id", "op_id", "state"}, []string{worktreeID, operationID, "cancel_requested"},
			), nil
		},
	}
}
