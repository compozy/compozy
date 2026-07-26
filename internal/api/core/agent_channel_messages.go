package core

import (
	"encoding/json"

	"sort"

	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network"

	"github.com/compozy/agh/internal/store"
)

func agentChannelMessagesFromEnvelopes(
	envelopes []network.Envelope,
	channel string,
	limit int,
) []contract.AgentChannelMessagePayload {
	filtered := filterAgentChannelEnvelopes(envelopes, channel)
	messages := make([]contract.AgentChannelMessagePayload, 0, len(filtered))
	for _, envelope := range filtered {
		metadata, ok := coordinationMetadataFromEnvelope(envelope)
		if !ok {
			continue
		}
		if err := contract.ValidateNoRawClaimTokenField(struct {
			Body     json.RawMessage                             `json:"body"`
			Metadata contract.CoordinationMessageMetadataPayload `json:"metadata"`
		}{Body: envelope.Body, Metadata: metadata}); err != nil {
			continue
		}
		messages = append(messages, contract.AgentChannelMessagePayload{
			MessageID: strings.TrimSpace(envelope.ID),
			ChannelID: firstNonEmpty(
				strings.TrimSpace(metadata.ChannelID),
				strings.TrimSpace(envelope.Channel),
			),
			FromSessionID: strings.TrimSpace(envelope.From),
			ToSessionID:   stringPtrValue(envelope.To),
			Body:          cloneRawMessage(envelope.Body),
			Metadata:      metadata,
			Timestamp:     envelopeTime(envelope),
		})
	}
	sort.SliceStable(messages, func(left, right int) bool {
		if !messages[left].Timestamp.Equal(messages[right].Timestamp) {
			return messages[left].Timestamp.Before(messages[right].Timestamp)
		}
		return messages[left].MessageID < messages[right].MessageID
	})
	if limit > 0 && len(messages) > limit {
		return messages[:limit]
	}
	return messages
}

func agentChannelMessageFromRequest(
	messageID string,
	channel string,
	fromSessionID string,
	toSessionID string,
	body json.RawMessage,
	metadata contract.CoordinationMessageMetadataPayload,
	timestamp time.Time,
) contract.AgentChannelMessagePayload {
	return contract.AgentChannelMessagePayload{
		MessageID:     strings.TrimSpace(messageID),
		ChannelID:     firstNonEmpty(strings.TrimSpace(metadata.ChannelID), strings.TrimSpace(channel)),
		FromSessionID: strings.TrimSpace(fromSessionID),
		ToSessionID:   strings.TrimSpace(toSessionID),
		Body:          cloneRawMessage(body),
		Metadata:      metadata,
		Timestamp:     timestamp.UTC(),
	}
}

func filterAgentChannelEnvelopes(envelopes []network.Envelope, channel string) []network.Envelope {
	channel = strings.TrimSpace(channel)
	filtered := make([]network.Envelope, 0, len(envelopes))
	for _, envelope := range envelopes {
		if channel == "" || strings.TrimSpace(envelope.Channel) == channel {
			filtered = append(filtered, envelope)
		}
	}
	return filtered
}

func coordinationMetadataFromEnvelope(
	envelope network.Envelope,
) (contract.CoordinationMessageMetadataPayload, bool) {
	for _, key := range []string{agentCoordinationExtKey, "coordination_metadata", "agh_coordination", "metadata"} {
		if raw, ok := envelope.Ext[key]; ok {
			var metadata contract.CoordinationMessageMetadataPayload
			if err := json.Unmarshal(raw, &metadata); err == nil {
				return metadata, true
			}
		}
	}
	return contract.CoordinationMessageMetadataPayload{}, false
}

func envelopeFromNetworkMessage(entry store.NetworkMessageEntry) (network.Envelope, error) {
	envelope := network.Envelope{
		Protocol:    network.ProtocolV0,
		ID:          strings.TrimSpace(entry.MessageID),
		Kind:        network.Kind(strings.TrimSpace(entry.Kind)),
		WorkspaceID: strings.TrimSpace(entry.WorkspaceID),
		Channel:     strings.TrimSpace(entry.Channel),
		From:        strings.TrimSpace(entry.PeerFrom),
		Mentions:    cloneTrimmedStrings(entry.Mentions),
		WorkID:      optionalStringPtr(entry.WorkID),
		ReplyTo:     optionalStringPtr(entry.ReplyTo),
		TraceID:     optionalStringPtr(entry.TraceID),
		CausationID: optionalStringPtr(entry.CausationID),
		TS:          entry.Timestamp.Unix(),
		Body:        cloneRawMessage(entry.Body),
	}
	ext, err := extensionMapFromNetworkMessage(entry)
	if err != nil {
		return network.Envelope{}, err
	}
	if len(ext) > 0 {
		envelope.Ext = ext
	}
	if to := strings.TrimSpace(entry.PeerTo); to != "" {
		envelope.To = &to
	}
	return envelope, nil
}
