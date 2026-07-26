package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/spf13/cobra"
)

func writeCommandOutputWithJSONL[T any](cmd *cobra.Command, bundle outputBundle, items []T) error {
	mode, err := resolveOutputFormat(cmd)
	if err != nil {
		return err
	}
	if mode == OutputJSONL {
		return writeJSONLines(cmd, items)
	}
	return writeCommandOutput(cmd, bundle)
}

func networkThreadHumanRow(thread NetworkThreadRecord) []string {
	return []string{
		stringOrDash(thread.ThreadID),
		stringOrDash(thread.RootMessageID),
		stringOrDash(thread.OpenedByPeerID),
		strconv.Itoa(thread.MessageCount),
		strconv.Itoa(thread.ParticipantCount),
		strconv.Itoa(thread.OpenWorkCount),
		stringOrDash(formatTimePtr(thread.LastActivityAt)),
		stringOrDash(thread.LastMessagePreview),
	}
}

func networkThreadToonRow(thread NetworkThreadRecord) []string {
	return []string{
		thread.Channel,
		thread.ThreadID,
		thread.RootMessageID,
		thread.OpenedByPeerID,
		strconv.Itoa(thread.MessageCount),
		strconv.Itoa(thread.ParticipantCount),
		strconv.Itoa(thread.OpenWorkCount),
		formatTimePtr(thread.LastActivityAt),
		thread.LastMessagePreview,
	}
}

func networkThreadKeyValues(thread NetworkThreadRecord) []keyValue {
	return []keyValue{
		{Label: networkChannelValue, Value: stringOrDash(thread.Channel)},
		{Label: networkThreadIDValue, Value: stringOrDash(thread.ThreadID)},
		{Label: "Root Message", Value: stringOrDash(thread.RootMessageID)},
		{Label: networkTitleValue, Value: stringOrDash(thread.Title)},
		{Label: networkOpenedByValue, Value: stringOrDash(thread.OpenedByPeerID)},
		{Label: "Opened Session", Value: stringOrDash(thread.OpenedSessionID)},
		{Label: networkOpenedAtValue, Value: stringOrDash(formatTimePtr(thread.OpenedAt))},
		{Label: networkLastActivityValue, Value: stringOrDash(formatTimePtr(thread.LastActivityAt))},
		{Label: networkMessagesValue, Value: strconv.Itoa(thread.MessageCount)},
		{Label: "Participants", Value: strconv.Itoa(thread.ParticipantCount)},
		{Label: networkOpenWorkValue, Value: strconv.Itoa(thread.OpenWorkCount)},
		{Label: toolOperatorPreviewValue, Value: stringOrDash(thread.LastMessagePreview)},
	}
}

func networkDirectHumanRow(direct NetworkDirectRoomRecord) []string {
	return []string{
		stringOrDash(direct.DirectID),
		stringOrDash(direct.SessionA),
		stringOrDash(direct.SessionB),
		strconv.Itoa(direct.MessageCount),
		strconv.Itoa(direct.OpenWorkCount),
		stringOrDash(formatTimePtr(direct.LastActivityAt)),
		stringOrDash(direct.LastMessagePreview),
	}
}

func networkDirectToonRow(direct NetworkDirectRoomRecord) []string {
	return []string{
		direct.Channel,
		direct.DirectID,
		direct.SessionA,
		direct.SessionB,
		strconv.Itoa(direct.MessageCount),
		strconv.Itoa(direct.OpenWorkCount),
		formatTimePtr(direct.LastActivityAt),
		direct.LastMessagePreview,
	}
}

func networkDirectKeyValues(direct NetworkDirectRoomRecord) []keyValue {
	return []keyValue{
		{Label: networkChannelValue, Value: stringOrDash(direct.Channel)},
		{Label: networkDirectIDValue, Value: stringOrDash(direct.DirectID)},
		{Label: "Session A", Value: stringOrDash(direct.SessionA)},
		{Label: "Session B", Value: stringOrDash(direct.SessionB)},
		{Label: networkOpenedAtValue, Value: stringOrDash(formatTimePtr(direct.OpenedAt))},
		{Label: networkLastActivityValue, Value: stringOrDash(formatTimePtr(direct.LastActivityAt))},
		{Label: networkMessagesValue, Value: strconv.Itoa(direct.MessageCount)},
		{Label: networkOpenWorkValue, Value: strconv.Itoa(direct.OpenWorkCount)},
		{Label: toolOperatorPreviewValue, Value: stringOrDash(direct.LastMessagePreview)},
	}
}

func networkMessageHumanRow(message NetworkConversationMessageRecord) []string {
	return []string{
		stringOrDash(message.MessageID),
		stringOrDash(message.Surface),
		stringOrDash(message.ThreadID),
		stringOrDash(message.DirectID),
		stringOrDash(message.Kind),
		stringOrDash(message.Direction),
		stringOrDash(message.PeerFrom),
		stringOrDash(message.PeerTo),
		stringOrDash(message.WorkID),
		stringOrDash(formatTime(message.Timestamp)),
		stringOrDash(networkMessagePreview(message)),
	}
}

func networkMessageToonRow(message NetworkConversationMessageRecord) []string {
	return []string{
		message.MessageID,
		message.Channel,
		message.Surface,
		message.ThreadID,
		message.DirectID,
		message.Kind,
		message.Direction,
		message.PeerFrom,
		message.PeerTo,
		message.WorkID,
		formatTime(message.Timestamp),
		networkMessagePreview(message),
	}
}

func networkMessagePreview(message NetworkConversationMessageRecord) string {
	if trimmed := strings.TrimSpace(message.PreviewText); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(message.Text)
}

func networkKindMetricRows(metrics []NetworkKindMetricRecord) [][]string {
	rows := make([][]string, 0, len(metrics))
	for _, metric := range metrics {
		rows = append(rows, []string{
			metric.Kind,
			strconv.FormatInt(metric.Sent, 10),
			strconv.FormatInt(metric.Received, 10),
			strconv.FormatInt(metric.Rejected, 10),
			strconv.FormatInt(metric.Delivered, 10),
		})
	}
	return rows
}

func parseNetworkJSONValue(flagName string, raw string) (json.RawMessage, error) {
	payload, err := parseRequiredJSONRawMessage(raw)
	if errors.Is(err, errEmptyJSONFlag) {
		return nil, fmt.Errorf("cli: %s is required", flagName)
	}
	if err != nil {
		return nil, fmt.Errorf("cli: %s must be valid JSON: %w", flagName, err)
	}
	return payload, nil
}

func parseNetworkJSONObjectMap(flagName string, raw string) (map[string]json.RawMessage, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	if !strings.HasPrefix(trimmed, "{") {
		return nil, fmt.Errorf("cli: %s must be a JSON object", flagName)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil, fmt.Errorf("cli: decode %s: %w", flagName, err)
	}
	return payload, nil
}

func validateNetworkSendNoRawClaimToken(body json.RawMessage, ext map[string]json.RawMessage) error {
	payload := struct {
		Body json.RawMessage            `json:"body"`
		Ext  map[string]json.RawMessage `json:"ext,omitempty"`
	}{
		Body: body,
		Ext:  ext,
	}
	if err := contract.ValidateNoRawClaimTokenField(payload); err != nil {
		return fmt.Errorf(
			"cli: network_raw_token_rejected: --body/--ext must not contain raw claim_token fields: %w",
			err,
		)
	}
	return nil
}

func parseNetworkExpiresAt(raw string) (*int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	if unixSeconds, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return &unixSeconds, nil
	}

	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return nil, errors.New("cli: --expires-at must be unix seconds or RFC3339")
	}
	unixSeconds := parsed.UTC().Unix()
	return &unixSeconds, nil
}

func networkCompactExt(ext map[string]json.RawMessage) string {
	if len(ext) == 0 {
		return ""
	}
	payload, err := json.Marshal(ext)
	if err != nil {
		return ""
	}
	return compactJSON(payload)
}

func networkExtString(ext map[string]json.RawMessage, key string) string {
	if len(ext) == 0 {
		return ""
	}
	raw, ok := ext[strings.TrimSpace(key)]
	if !ok || len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(compactJSON(raw))
}

func formatUnixSeconds(value *int64) string {
	if value == nil {
		return ""
	}
	return time.Unix(*value, 0).UTC().Format(time.RFC3339)
}

func formatTimePtr(value *time.Time) string {
	if value == nil {
		return ""
	}
	return formatTime(*value)
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
