package cli

import (
	"fmt"
	"strconv"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

const (
	goalLiveKey         = "live"
	idempotencyKeyField = "idempotency_key"
	idempotencyKeyLabel = "Idempotency Key"
)

func sessionPromptBundle(record SessionPromptRecord) outputBundle {
	if record.Goal != nil {
		return goalCommandBundle(record.Prompt, *record.Goal)
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

type goalCommandOutput struct {
	contract.GoalCommandResult
	MessageID      string `json:"message_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

func goalCommandBundle(prompt SessionPromptResultRecord, result contract.GoalCommandResult) outputBundle {
	output := goalCommandOutput{
		GoalCommandResult: result,
		MessageID:         prompt.MessageID,
		IdempotencyKey:    prompt.IdempotencyKey,
	}
	return outputBundle{
		jsonValue: output,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, output)
		},
		human: func() (string, error) {
			return renderHumanSectionResult("Goal", goalCommandRows(prompt, result))
		},
		toon: func() (string, error) {
			return renderToonObject("goal", goalCommandFields(), goalCommandValues(prompt, result)), nil
		},
	}
}

func goalCommandRows(prompt SessionPromptResultRecord, result contract.GoalCommandResult) []keyValue {
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
	rows = append(rows,
		keyValue{Label: cliMessageIDValue, Value: stringOrDash(prompt.MessageID)},
		keyValue{Label: idempotencyKeyLabel, Value: stringOrDash(prompt.IdempotencyKey)},
	)
	return rows
}

func goalCommandFields() []string {
	return []string{
		"outcome", "reason_code", "replaced_run_id", agentKernelRunIDKey, sessionStatusKey,
		"objective", "turns_used", "turn_limit", goalLiveKey, messageIDKey, idempotencyKeyField,
	}
}

func goalCommandValues(prompt SessionPromptResultRecord, result contract.GoalCommandResult) []string {
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
	values[9] = prompt.MessageID
	values[10] = prompt.IdempotencyKey
	return values
}

func sessionPromptRows(result SessionPromptResultRecord) []keyValue {
	rows := []keyValue{
		{Label: sessionStatusValue, Value: stringOrDash(result.Status)},
	}
	if result.Mode != "" {
		rows = append(rows, keyValue{Label: bridgeModeValue, Value: string(result.Mode)})
	}
	if result.Delivery != "" {
		rows = append(rows, keyValue{Label: cliDeliveryValue, Value: string(result.Delivery)})
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
	return rows
}

func sessionPromptFields() []string {
	return []string{
		sessionStatusKey,
		bridgeModeKey,
		cliDeliveryKey,
		"queue_entry_id",
		"queue_position",
		"queue_generation",
		"previous_turn_id",
		"new_turn_id",
		"canceled_queued_entries",
	}
}

func sessionPromptValues(result SessionPromptResultRecord) []string {
	return []string{
		result.Status,
		string(result.Mode),
		string(result.Delivery),
		result.QueueEntryID,
		strconv.Itoa(result.QueuePosition),
		strconv.FormatInt(result.QueueGeneration, 10),
		result.PreviousTurnID,
		result.NewTurnID,
		strconv.Itoa(result.CanceledQueuedEntries),
	}
}
