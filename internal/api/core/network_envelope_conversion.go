package core

import (
	"encoding/json"

	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network"
)

// NetworkChannelPayloadsFromInfos converts active channel summaries into shared payloads.
func NetworkChannelPayloadsFromInfos(channels []network.ChannelInfo) []contract.NetworkChannelPayload {
	payload := make([]contract.NetworkChannelPayload, 0, len(channels))
	for _, channel := range channels {
		payload = append(payload, contract.NetworkChannelPayload{
			WorkspaceID: strings.TrimSpace(channel.WorkspaceID),
			Channel:     channel.Channel,
			PeerCount:   channel.PeerCount,
		})
	}
	return payload
}

// NetworkEnvelopePayloadsFromEnvelopes converts surfaced envelopes into shared payloads.
func NetworkEnvelopePayloadsFromEnvelopes(envelopes []network.Envelope) []contract.NetworkEnvelopePayload {
	payload := make([]contract.NetworkEnvelopePayload, 0, len(envelopes))
	for _, envelope := range envelopes {
		payload = append(payload, NetworkEnvelopePayloadFromEnvelope(envelope))
	}
	return payload
}

// NetworkEnvelopePayloadFromEnvelope converts one surfaced envelope into the shared payload.
func NetworkEnvelopePayloadFromEnvelope(envelope network.Envelope) contract.NetworkEnvelopePayload {
	return contract.NetworkEnvelopePayload{
		Protocol:    envelope.Protocol,
		ID:          envelope.ID,
		Kind:        string(envelope.Kind),
		WorkspaceID: strings.TrimSpace(envelope.WorkspaceID),
		Channel:     envelope.Channel,
		Surface:     cloneSurfacePtr(envelope.Surface),
		ThreadID:    cloneStringPtr(envelope.ThreadID),
		DirectID:    cloneStringPtr(envelope.DirectID),
		From:        envelope.From,
		To:          cloneStringPtr(envelope.To),
		Mentions:    cloneTrimmedStrings(envelope.Mentions),
		WorkID:      cloneStringPtr(envelope.WorkID),
		ReplyTo:     cloneStringPtr(envelope.ReplyTo),
		TraceID:     cloneStringPtr(envelope.TraceID),
		CausationID: cloneStringPtr(envelope.CausationID),
		TS:          envelope.TS,
		ExpiresAt:   cloneInt64Ptr(envelope.ExpiresAt),
		Body:        cloneRawMessage(envelope.Body),
		Proof:       cloneProofPtr(envelope.Proof),
		Ext:         cloneRawMap(envelope.Ext),
	}
}

func cloneSurfacePtr(value *network.Surface) *string {
	if value == nil {
		return nil
	}
	copyValue := strings.TrimSpace(string(*value))
	return &copyValue
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	copyValue := strings.TrimSpace(*value)
	return &copyValue
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copyValue := value.UTC()
	return &copyValue
}

func ptrString(value string) *string {
	copyValue := strings.TrimSpace(value)
	return &copyValue
}

func cloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func cloneRawMap[T ~map[string]json.RawMessage](source T) map[string]json.RawMessage {
	if len(source) == 0 {
		return nil
	}
	cloned := make(map[string]json.RawMessage, len(source))
	for key, value := range source {
		cloned[key] = cloneRawMessage(value)
	}
	return cloned
}

func cloneProofPtr(source *network.Proof) map[string]json.RawMessage {
	if source == nil {
		return nil
	}
	return cloneRawMap(*source)
}
