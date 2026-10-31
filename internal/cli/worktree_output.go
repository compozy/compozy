package cli

import (
	"strconv"

	"github.com/compozy/compozy/internal/api/contract"
)

func worktreeRecordBundle(item WorktreeRecord) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanSection("Worktree", []keyValue{
				{Label: "ID", Value: stringOrDash(item.ID)},
				{Label: sessionProfileValue, Value: stringOrDash(item.ProfileName)},
				{Label: automationNameValue, Value: stringOrDash(item.Name)},
				{Label: worktreeBranchLabel, Value: stringOrDash(item.Branch)},
				{Label: "Path", Value: stringOrDash(item.Path)},
				{Label: authoredContextStateValue, Value: stringOrDash(item.State)},
				{Label: taskOriginValue, Value: stringOrDash(item.Origin)},
				{Label: "Agent Activity", Value: stringOrDash(item.AgentActivity)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"worktree",
				[]string{"id", profileNameOutputKey, automationNameKey, worktreeBranchKey, automationPathKey, stateKey,
					taskOriginKey, "agent_activity"},
				[]string{
					item.ID,
					item.ProfileName,
					item.Name,
					item.Branch,
					item.Path,
					item.State,
					item.Origin,
					item.AgentActivity,
				},
			), nil
		},
	}
}

func worktreeListBundle(items []WorktreeRecord) outputBundle {
	if items == nil {
		items = []WorktreeRecord{}
	}
	return listBundle(
		items,
		items,
		"Worktrees",
		[]string{"ID", sessionProfileValue, automationNameValue, worktreeBranchLabel, authoredContextStateValue,
			"Dirty", "Ahead", "Behind", authoredContextAgentValue},
		"worktrees",
		[]string{"id", profileNameOutputKey, automationNameKey, worktreeBranchKey, automationPathKey, stateKey,
			taskOriginKey, "dirty", "ahead", "behind", "agent_activity"},
		func(item WorktreeRecord) []string {
			return []string{
				item.ID, item.ProfileName, item.Name, item.Branch, item.State, formatOptionalBool(item.Dirty),
				formatOptionalInt(item.Ahead), formatOptionalInt(item.Behind), item.AgentActivity,
			}
		},
		func(item WorktreeRecord) []string {
			return []string{
				item.ID, item.ProfileName, item.Name, item.Branch, item.Path, item.State, item.Origin,
				formatOptionalBool(item.Dirty), formatOptionalInt(item.Ahead),
				formatOptionalInt(item.Behind), item.AgentActivity,
			}
		},
	)
}

func worktreeInspectBundle(item WorktreeInspectRecord) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			table, err := worktreeRecordBundle(item.Worktree).human()
			if err != nil || item.Worktree.CreateError == "" {
				return table, err
			}
			failure, err := renderHumanSectionResult("Creation Failure", []keyValue{
				{Label: "Phase", Value: stringOrDash(item.Worktree.PendingPhase)},
				{Label: "Cause", Value: item.Worktree.CreateError},
			})
			return table + "\n" + failure, err
		},
		toon: func() (string, error) {
			return worktreeRecordBundle(item.Worktree).toon()
		},
	}
}

func worktreeStatusBundle(item WorktreeStatusRecord) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanSection("Worktree Status", []keyValue{
				{Label: worktreeLabel, Value: stringOrDash(item.WorktreeID)},
				{Label: "Branch", Value: stringOrDash(pointerStringValue(item.Status.Branch))},
				{Label: "Dirty Files", Value: formatOptionalInt(item.Status.DirtyFiles)},
				{Label: "Ahead", Value: formatOptionalInt(item.Status.Ahead)},
				{Label: "Behind", Value: formatOptionalInt(item.Status.Behind)},
				{Label: "Read Error", Value: stringOrDash(item.Status.ReadError)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject("worktree_status",
				[]string{worktreeIDKey, worktreeBranchKey, "dirty_files", "ahead", "behind", "read_error"},
				[]string{item.WorktreeID, pointerStringValue(item.Status.Branch),
					formatOptionalInt(item.Status.DirtyFiles), formatOptionalInt(item.Status.Ahead),
					formatOptionalInt(item.Status.Behind), item.Status.ReadError},
			), nil
		},
	}
}

// worktreeMutationBundle reports a lifecycle mutation; branch is present only
// when the caller opted into local branch deletion.
func worktreeMutationBundle(
	action string,
	item WorktreeRecord,
	branch *contract.WorktreeBranchOutcomePayload,
) outputBundle {
	type result struct {
		Action   string                                 `json:"action"`
		Worktree WorktreeRecord                         `json:"worktree"`
		Branch   *contract.WorktreeBranchOutcomePayload `json:"branch,omitempty"`
	}
	value := result{Action: action, Worktree: item, Branch: branch}
	rows := []keyValue{
		{Label: "Action", Value: action},
		{Label: "ID", Value: stringOrDash(item.ID)},
		{Label: automationNameValue, Value: stringOrDash(item.Name)},
		{Label: authoredContextStateValue, Value: stringOrDash(item.State)},
	}
	keys := []string{authoredContextActionKey, "id", automationNameKey, stateKey}
	values := []string{action, item.ID, item.Name, item.State}
	if branch != nil {
		summary := worktreeBranchOutcomeSummary(*branch)
		rows = append(rows, keyValue{Label: "Branch", Value: summary})
		keys = append(keys, "branch")
		values = append(values, summary)
	}
	return outputBundle{
		jsonValue: value,
		human: func() (string, error) {
			return renderHumanSection("Worktree", rows), nil
		},
		toon: func() (string, error) {
			return renderToonObject("worktree", keys, values), nil
		},
	}
}

func worktreeBranchOutcomeSummary(branch contract.WorktreeBranchOutcomePayload) string {
	name := stringOrDash(branch.Name)
	if branch.Deleted {
		return name + " deleted"
	}
	summary := name + " kept (" + branch.Reason + ")"
	if branch.Detail != "" {
		summary += ": " + branch.Detail
	}
	return summary
}

func formatOptionalInt(value *int) string {
	if value == nil {
		return "-"
	}
	return strconv.Itoa(*value)
}

func formatOptionalBool(value *bool) string {
	if value == nil {
		return "-"
	}
	return strconv.FormatBool(*value)
}

func pointerStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
