package cli

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func callCreateBundle(record contract.CallCreatePayload) outputBundle {
	return recordBundle(record, "Call", []keyValue{
		{Label: "Call", Value: stringOrDash(record.CallID)},
		{Label: "Child", Value: stringOrDash(record.ChildSessionID)},
		{Label: "State", Value: stringOrDash(record.State)},
		{Label: "Replayed", Value: fmt.Sprintf("%t", record.Replayed)},
	})
}

func callDetailBundle(record contract.CallPayload) outputBundle {
	return recordBundle(record, "Call", callDetailRows(record))
}

func callListBundle(response contract.CallsResponse) outputBundle {
	return listBundle(
		response,
		response.Items,
		"Calls",
		[]string{"CALL", "AGENT", "CHILD", "STATE", "RESULT"},
		"calls",
		[]string{"call_id", "agent", "child_session_id", "state", "verdict"},
		func(item contract.CallPayload) []string {
			result := stringOrDash(item.Verdict)
			if item.ResultBytes > 0 {
				result = fmt.Sprintf("%s (%d B)", result, item.ResultBytes)
			}
			return []string{item.CallID, item.Agent, item.ChildSessionID, item.State, result}
		},
		func(item contract.CallPayload) []string {
			return []string{item.CallID, item.Agent, item.ChildSessionID, item.State, item.Verdict}
		},
	)
}

func callResultBundle(result contract.CallResultResponse) outputBundle {
	return outputBundle{
		jsonValue: result.Result,
		json: func(cmd *cobra.Command) error {
			return writeJSONWithoutWorkspaceResolution(cmd, result.Result)
		},
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLineWithoutWorkspaceResolution(cmd, result.Result)
		},
		human: func() (string, error) { return strings.TrimSpace(string(result.Result)), nil },
		toon:  func() (string, error) { return strings.TrimSpace(string(result.Result)), nil },
	}
}

func callAwaitBundle(response contract.AwaitCallsResponse) outputBundle {
	rows := []keyValue{
		{Label: "Outcome", Value: stringOrDash(response.Outcome)},
		{Label: "Pending", Value: fmt.Sprintf("%d", len(response.Pending))},
		{Label: "Settled", Value: fmt.Sprintf("%d", len(response.Settled))},
		{Label: "Resume", Value: stringOrDash(response.Resume)},
	}
	return recordBundle(response, "Await", rows)
}

func callCancelBundle(response contract.CancelCallResponse) outputBundle {
	return recordBundle(response, "Call", []keyValue{{Label: "State", Value: stringOrDash(response.State)}})
}

func callPublishBundle(response contract.PublishCallResponse) outputBundle {
	return recordBundle(response, "Publication", []keyValue{
		{Label: "Published", Value: fmt.Sprintf("%t", response.Published)},
		{Label: "Message", Value: stringOrDash(response.NetworkMessageID)},
	})
}

func messageSendBundle(response contract.SendCallMessageResponse) outputBundle {
	return recordBundle(response, "Message", []keyValue{
		{Label: "Message", Value: stringOrDash(response.MessageID)},
		{Label: "Delivery", Value: stringOrDash(response.Delivery)},
	})
}

func messageListBundle(response contract.CallMessagesResponse) outputBundle {
	return listBundle(
		response,
		response.Items,
		"Messages",
		[]string{"MESSAGE", "FROM", "TO", "DELIVERY", "TEXT"},
		"messages",
		[]string{"message_id", "from", "to_session_id", "delivery", "text"},
		func(item contract.CallMessagePayload) []string {
			from := item.From.ID
			if item.From.Kind == "operator" {
				from = "operator"
			}
			return []string{item.MessageID, from, item.ToSessionID, item.Delivery, item.Text}
		},
		func(item contract.CallMessagePayload) []string {
			return []string{item.MessageID, item.From.ID, item.ToSessionID, item.Delivery, item.Text}
		},
	)
}

func recordBundle(value any, title string, rows []keyValue) outputBundle {
	return outputBundle{
		jsonValue: value,
		jsonl:     func(cmd *cobra.Command) error { return writeJSONLine(cmd, value) },
		human:     func() (string, error) { return renderHumanSection(title, rows), nil },
		toon:      func() (string, error) { return renderJSONPreview(value) },
	}
}

func callDetailRows(record contract.CallPayload) []keyValue {
	rows := []keyValue{
		{Label: "Call", Value: stringOrDash(record.CallID)},
		{Label: "Agent", Value: stringOrDash(record.Agent)},
		{Label: "Caller", Value: stringOrDash(record.Caller.ID)},
		{Label: "Child", Value: stringOrDash(record.ChildSessionID)},
		{Label: "State", Value: stringOrDash(record.State)},
		{Label: "Verdict", Value: stringOrDash(record.Verdict)},
		{Label: "Contract", Value: stringOrDash(record.ExpectDigest)},
		{Label: "Result", Value: fmt.Sprintf("%d B", record.ResultBytes)},
	}
	if strings.TrimSpace(record.FinalProsePreview) != "" {
		rows = append(rows, keyValue{Label: "Prose", Value: record.FinalProsePreview})
	}
	return rows
}
