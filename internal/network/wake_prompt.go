package network

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
)

const networkWakeTrustUntrusted = "untrusted"

const (
	networkWakePromptMaxMessages = 32
	networkWakePromptMaxBytes    = 64 * 1024
	networkWakeFetchInstruction  = "Read the committed inbox message by message_id for full content."
)

type networkWakePromptRecord struct {
	MessageID        string          `json:"message_id"`
	From             string          `json:"from"`
	Kind             string          `json:"kind"`
	Body             json.RawMessage `json:"body,omitempty"`
	BodyPreview      string          `json:"body_preview,omitempty"`
	BodyOmitted      bool            `json:"body_omitted,omitempty"`
	FetchInstruction string          `json:"fetch_instruction,omitempty"`
}

// FormatNetworkWakePrompt renders only committed message content plus a compact wake header.
func FormatNetworkWakePrompt(
	messages []store.NetworkMessageEntry,
	targetSessionID string,
) (string, acp.PromptNetworkMeta, error) {
	if len(messages) == 0 {
		return "", acp.PromptNetworkMeta{}, errors.New("network: wake requires at least one message")
	}
	target := strings.TrimSpace(targetSessionID)
	if target == "" {
		return "", acp.PromptNetworkMeta{}, errors.New("network: wake target session id is required")
	}
	selectedCount := min(len(messages), networkWakePromptMaxMessages)
	var builder strings.Builder
	if selectedCount < len(messages) {
		fmt.Fprintf(
			&builder,
			"Network wake: %d committed message(s); showing %d of %d.\n",
			len(messages),
			selectedCount,
			len(messages),
		)
	} else {
		fmt.Fprintf(&builder, "Network wake: %d committed message(s).\n", len(messages))
	}
	for index, message := range messages[:selectedCount] {
		body, err := DecodeBody(Kind(message.Kind), message.Body)
		if err != nil {
			return "", acp.PromptNetworkMeta{}, fmt.Errorf(
				"network: decode wake message %q: %w",
				message.MessageID,
				err,
			)
		}
		remainingMessages := selectedCount - index
		remainingBytes := networkWakePromptMaxBytes - builder.Len()
		line, err := renderNetworkWakePromptRecord(message, body, remainingBytes/remainingMessages)
		if err != nil {
			return "", acp.PromptNetworkMeta{}, err
		}
		builder.WriteString(line)
	}
	first := messages[0]
	meta := acp.PromptNetworkMeta{
		MessageID:    first.MessageID,
		Kind:         first.Kind,
		Channel:      first.Channel,
		Surface:      first.Surface,
		ThreadID:     first.ThreadID,
		DirectID:     first.DirectID,
		From:         first.PeerFrom,
		To:           first.PeerTo,
		Mentions:     append([]string(nil), first.Mentions...),
		WorkID:       first.WorkID,
		ReplyTo:      first.ReplyTo,
		TraceID:      first.TraceID,
		CausationID:  first.CausationID,
		Trust:        networkWakeTrustUntrusted,
		DeliveryMode: wakeDeliveryMode(first, target),
	}
	return strings.TrimSpace(builder.String()), meta.Normalize(), nil
}

func renderNetworkWakePromptRecord(
	message store.NetworkMessageEntry,
	body Body,
	maxBytes int,
) (string, error) {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("network: encode wake message %q body: %w", message.MessageID, err)
	}
	record := networkWakePromptRecord{
		MessageID: strings.TrimSpace(message.MessageID),
		From:      strings.TrimSpace(message.PeerFrom),
		Kind:      strings.TrimSpace(message.Kind),
		Body:      bodyJSON,
	}
	line, err := marshalNetworkWakePromptRecord(record)
	if err != nil {
		return "", err
	}
	if len(line) <= maxBytes {
		return line, nil
	}
	record.Body = nil
	record.BodyPreview = previewForBody(body)
	record.BodyOmitted = true
	record.FetchInstruction = networkWakeFetchInstruction
	line, err = marshalNetworkWakePromptRecord(record)
	if err != nil {
		return "", err
	}
	if len(line) > maxBytes {
		return "", fmt.Errorf(
			"network: bounded wake record %q requires %d bytes, budget is %d",
			record.MessageID,
			len(line),
			maxBytes,
		)
	}
	return line, nil
}

func marshalNetworkWakePromptRecord(record networkWakePromptRecord) (string, error) {
	encoded, err := json.Marshal(record)
	if err != nil {
		return "", fmt.Errorf("network: encode wake message %q: %w", record.MessageID, err)
	}
	return string(encoded) + "\n", nil
}

func wakeDeliveryMode(message store.NetworkMessageEntry, targetSessionID string) string {
	if strings.TrimSpace(message.PeerTo) == strings.TrimSpace(targetSessionID) {
		return store.NetworkWakeTriggerDirect
	}
	return store.NetworkWakeTriggerMention
}
