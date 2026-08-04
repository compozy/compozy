package cli

import (
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func sessionInputListBundle(response SessionInputListRecord) outputBundle {
	return listBundle(
		response,
		response.Inputs,
		"Pending Session Input",
		[]string{"ID", "MODE", "STATUS", "DELIVERY", "TARGET TURN", "TEXT", "QUEUED AT"},
		"session_inputs",
		[]string{
			"id", bridgeModeKey, automationStatusKey, cliDeliveryKey, "target_turn_id", sessionClarifyTextFlag,
			"enqueued_at",
		},
		func(input SessionInputRecord) []string {
			return []string{
				stringOrDash(input.ID), stringOrDash(string(input.Mode)), stringOrDash(input.Status),
				stringOrDash(string(input.Delivery)), stringOrDash(input.TargetTurnID),
				stringOrDash(input.Text), formatTime(input.EnqueuedAt),
			}
		},
		func(input SessionInputRecord) []string {
			return []string{
				input.ID, string(input.Mode), input.Status, string(input.Delivery), input.TargetTurnID,
				input.Text, formatTime(input.EnqueuedAt),
			}
		},
	)
}

func sessionInputRecordBundle(input SessionInputRecord) outputBundle {
	response := contract.SessionInputResponse{Input: input}
	return outputBundle{
		jsonValue: response,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, input)
		},
		human: func() (string, error) {
			return renderHumanSectionResult("Session Input", sessionInputRows(input))
		},
		toon: func() (string, error) {
			return renderToonObject("session_input", sessionInputFields(), sessionInputValues(input)), nil
		},
	}
}

func sessionInputRows(input SessionInputRecord) []keyValue {
	rows := []keyValue{
		{Label: "Input ID", Value: stringOrDash(input.ID)},
		{Label: "Session", Value: stringOrDash(input.SessionID)},
		{Label: cliMessageIDValue, Value: stringOrDash(input.MessageID)},
		{Label: idempotencyKeyLabel, Value: stringOrDash(input.IdempotencyKey)},
		{Label: "Target Turn", Value: stringOrDash(input.TargetTurnID)},
		{Label: automationStatusValue, Value: stringOrDash(input.Status)},
		{Label: "Mode", Value: stringOrDash(string(input.Mode))},
		{Label: cliDeliveryValue, Value: stringOrDash(string(input.Delivery))},
		{Label: "Text", Value: stringOrDash(input.Text)},
		{Label: "Queue Generation", Value: strconv.FormatInt(input.QueueGeneration, 10)},
		{Label: "Enqueued At", Value: formatTime(input.EnqueuedAt)},
	}
	if runtime := sessionInputRuntime(input.Runtime); runtime != "" {
		rows = append(rows, keyValue{Label: cliRuntimeValue, Value: runtime})
	}
	return rows
}

func sessionInputFields() []string {
	return []string{
		"id", bridgeSessionIDKey, "message_id", "idempotency_key", "target_turn_id", automationStatusKey,
		bridgeModeKey, cliDeliveryKey,
		sessionClarifyTextFlag, "queue_generation", "enqueued_at", cliRuntimeKey,
	}
}

func sessionInputValues(input SessionInputRecord) []string {
	return []string{
		input.ID, input.SessionID, input.MessageID, input.IdempotencyKey, input.TargetTurnID, input.Status,
		string(input.Mode), string(input.Delivery), input.Text, strconv.FormatInt(input.QueueGeneration, 10),
		formatTime(input.EnqueuedAt), sessionInputRuntime(input.Runtime),
	}
}

func sessionInputRuntime(runtime *contract.PromptRuntimeSelectionPayload) string {
	if runtime == nil {
		return ""
	}
	parts := []string{strings.TrimSpace(runtime.Provider), strings.TrimSpace(runtime.Model)}
	if effort := strings.TrimSpace(string(runtime.ReasoningEffort)); effort != "" {
		parts = append(parts, effort)
	}
	if speed := strings.TrimSpace(string(runtime.Speed)); speed != "" {
		parts = append(parts, speed)
	}
	return strings.TrimSpace(strings.Join(parts, " / "))
}
