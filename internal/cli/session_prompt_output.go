package cli

import (
	"fmt"
	"strconv"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/spf13/cobra"
)

const goalLiveKey = "live"

func sessionPromptBundle(record SessionPromptRecord) outputBundle {
	if record.Goal != nil {
		return goalCommandBundle(*record.Goal)
	}
	if record.Prompt.Status == "" && len(record.Events) > 0 {
		return agentEventsBundle(record.Events)
	}
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return renderHumanSectionResult("Prompt", sessionPromptRows(record.Prompt))
		},
		toon: func() (string, error) {
			return renderToonObject("prompt", sessionPromptFields(), sessionPromptValues(record.Prompt)), nil
		},
	}
}

func goalCommandBundle(result contract.GoalCommandResult) outputBundle {
	return outputBundle{
		jsonValue: result,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, result)
		},
		human: func() (string, error) {
			return renderHumanSectionResult("Goal", goalCommandRows(result))
		},
		toon: func() (string, error) {
			return renderToonObject("goal", goalCommandFields(), goalCommandValues(result)), nil
		},
	}
}

func goalCommandRows(result contract.GoalCommandResult) []keyValue {
	rows := []keyValue{{Label: taskOutcomeValue, Value: stringOrDash(string(result.Outcome))}}
	if result.ReasonCode != nil {
		rows = append(rows, keyValue{Label: "Reason", Value: string(*result.ReasonCode)})
	}
	if result.ReplacedRunID != nil {
		rows = append(rows, keyValue{Label: "Replaced Run", Value: *result.ReplacedRunID})
	}
	if result.Snapshot != nil {
		rows = append(rows,
			keyValue{Label: "Run", Value: result.Snapshot.RunID},
			keyValue{Label: sessionStatusValue, Value: string(result.Snapshot.Status)},
			keyValue{Label: "Objective", Value: result.Snapshot.Objective},
			keyValue{
				Label: cliTurnsValue,
				Value: fmt.Sprintf("%d/%d", result.Snapshot.TurnsUsed, result.Snapshot.TurnLimit),
			},
		)
	}
	return rows
}

func goalCommandFields() []string {
	return []string{
		"outcome", "reason_code", "replaced_run_id", agentKernelRunIDKey, sessionStatusKey,
		"objective", "turns_used", "turn_limit", goalLiveKey,
	}
}

func goalCommandValues(result contract.GoalCommandResult) []string {
	values := make([]string, len(goalCommandFields()))
	values[0] = string(result.Outcome)
	if result.ReasonCode != nil {
		values[1] = string(*result.ReasonCode)
	}
	if result.ReplacedRunID != nil {
		values[2] = *result.ReplacedRunID
	}
	if result.Snapshot != nil {
		values[3] = result.Snapshot.RunID
		values[4] = string(result.Snapshot.Status)
		values[5] = result.Snapshot.Objective
		values[6] = strconv.Itoa(result.Snapshot.TurnsUsed)
		values[7] = strconv.Itoa(result.Snapshot.TurnLimit)
		values[8] = strconv.FormatBool(result.Snapshot.Live)
	}
	return values
}

func sessionPromptRows(result SessionPromptResultRecord) []keyValue {
	rows := []keyValue{
		{Label: sessionStatusValue, Value: stringOrDash(result.Status)},
	}
	if result.Mode != "" {
		rows = append(rows, keyValue{Label: bridgeModeValue, Value: string(result.Mode)})
	}
	if result.QueueEntryID != "" {
		rows = append(rows, keyValue{Label: "Queue Entry", Value: result.QueueEntryID})
	}
	if result.QueuePosition > 0 {
		rows = append(rows, keyValue{Label: "Queue Position", Value: strconv.Itoa(result.QueuePosition)})
	}
	if result.QueueGeneration > 0 {
		rows = append(rows, keyValue{Label: "Queue Generation", Value: strconv.FormatInt(result.QueueGeneration, 10)})
	}
	if result.PreviousTurnID != "" {
		rows = append(rows, keyValue{Label: "Previous Turn", Value: result.PreviousTurnID})
	}
	if result.NewTurnID != "" {
		rows = append(rows, keyValue{Label: "New Turn", Value: result.NewTurnID})
	}
	if result.CanceledQueuedEntries > 0 {
		rows = append(rows, keyValue{Label: "Canceled Queued", Value: strconv.Itoa(result.CanceledQueuedEntries)})
	}
	if result.FallbackModeIfNoToolResult != "" {
		rows = append(rows, keyValue{Label: "Fallback Mode", Value: string(result.FallbackModeIfNoToolResult)})
	}
	rows = append(rows,
		keyValue{Label: "Queued", Value: strconv.FormatBool(result.Queued)},
		keyValue{Label: "Staged", Value: strconv.FormatBool(result.Staged)},
		keyValue{Label: "Interrupted", Value: strconv.FormatBool(result.Interrupted)},
	)
	return rows
}

func sessionPromptFields() []string {
	return []string{
		sessionStatusKey,
		bridgeModeKey,
		"queued",
		"staged",
		"interrupted",
		"queue_entry_id",
		"queue_position",
		"queue_generation",
		"previous_turn_id",
		"new_turn_id",
		"canceled_queued_entries",
		"fallback_mode_if_no_tool_result",
	}
}

func sessionPromptValues(result SessionPromptResultRecord) []string {
	return []string{
		result.Status,
		string(result.Mode),
		strconv.FormatBool(result.Queued),
		strconv.FormatBool(result.Staged),
		strconv.FormatBool(result.Interrupted),
		result.QueueEntryID,
		strconv.Itoa(result.QueuePosition),
		strconv.FormatInt(result.QueueGeneration, 10),
		result.PreviousTurnID,
		result.NewTurnID,
		strconv.Itoa(result.CanceledQueuedEntries),
		string(result.FallbackModeIfNoToolResult),
	}
}
