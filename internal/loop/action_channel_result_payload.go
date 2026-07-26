package loop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	storepkg "github.com/compozy/agh/internal/store"
)

type channelResultSendMeta struct {
	MessageID   string `json:"message_id"`
	WorkspaceID string `json:"workspace_id"`
	Channel     string `json:"channel"`
	Surface     string `json:"surface"`
	ThreadID    string `json:"thread_id"`
	DirectID    string `json:"direct_id"`
	WorkID      string `json:"work_id"`
	ID          string `json:"id"`
}

func channelResultSendMetadata(raw ActionRawResult) (channelResultSendMeta, error) {
	var params channelResultSendMeta
	if len(bytes.TrimSpace(raw.RenderedParams)) > 0 {
		if err := json.Unmarshal(raw.RenderedParams, &params); err != nil {
			return channelResultSendMeta{}, fmt.Errorf(
				"%w: decode channel_result network send params: %w",
				ErrValidation,
				err,
			)
		}
	}
	params.WorkspaceID = firstNonEmpty(params.WorkspaceID, raw.WorkspaceID)
	params.MessageID = firstNonEmpty(channelResultMessageID(raw.Structured), params.ID)
	if strings.TrimSpace(params.MessageID) == "" {
		return channelResultSendMeta{}, fmt.Errorf("%w: channel_result requires sent network message_id", ErrValidation)
	}
	return params.normalized(), nil
}

func channelResultMessageID(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	var payload struct {
		MessageID string `json:"message_id"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.MessageID)
}

func (m channelResultSendMeta) normalized() channelResultSendMeta {
	return channelResultSendMeta{
		MessageID:   strings.TrimSpace(m.MessageID),
		WorkspaceID: strings.TrimSpace(m.WorkspaceID),
		Channel:     strings.TrimSpace(m.Channel),
		Surface:     strings.TrimSpace(m.Surface),
		ThreadID:    strings.TrimSpace(m.ThreadID),
		DirectID:    strings.TrimSpace(m.DirectID),
		WorkID:      strings.TrimSpace(m.WorkID),
		ID:          strings.TrimSpace(m.ID),
	}
}

func (m channelResultSendMeta) conversationRef() (storepkg.NetworkConversationRef, error) {
	ref := storepkg.NetworkConversationRef{
		WorkspaceID: m.WorkspaceID,
		Channel:     m.Channel,
		Surface:     m.Surface,
		ThreadID:    m.ThreadID,
		DirectID:    m.DirectID,
	}
	if err := ref.Validate(); err != nil {
		return storepkg.NetworkConversationRef{}, fmt.Errorf(
			"%w: invalid channel_result conversation ref: %w",
			ErrValidation,
			err,
		)
	}
	return ref, nil
}

type channelResultPayload struct {
	structured json.RawMessage
	text       string
}

type channelResultSayBody struct {
	Text   string          `json:"text"`
	Intent string          `json:"intent"`
	Result json.RawMessage `json:"result"`
}

type channelResultTraceBody struct {
	State   string          `json:"state"`
	Message string          `json:"message"`
	Result  json.RawMessage `json:"result"`
}

func channelResultFromMessage(
	message storepkg.NetworkConversationMessage,
	req *ChannelResultHarvestRequest,
) (ChannelResultHarvestResult, bool, error) {
	if responder := strings.TrimSpace(req.Responder); responder != "" {
		if strings.TrimSpace(message.PeerFrom) != responder {
			return ChannelResultHarvestResult{}, false, nil
		}
	}
	payload, ok, err := extractChannelResultPayload(message)
	if err != nil {
		return ChannelResultHarvestResult{}, false, err
	}
	if !ok {
		return ChannelResultHarvestResult{}, false, nil
	}
	matched, err := channelResultContentRuleMatches(req.ContentRule, payload)
	if err != nil {
		return ChannelResultHarvestResult{}, false, err
	}
	if !matched {
		return ChannelResultHarvestResult{}, false, nil
	}
	return ChannelResultHarvestResult{
		Found:      true,
		MessageID:  strings.TrimSpace(message.MessageID),
		Structured: cloneRawMessage(payload.structured),
		Text:       payload.text,
	}, true, nil
}

func extractChannelResultPayload(
	message storepkg.NetworkConversationMessage,
) (channelResultPayload, bool, error) {
	switch strings.TrimSpace(message.Kind) {
	case storepkg.NetworkKindSay:
		return extractSayChannelResultPayload(message)
	case storepkg.NetworkKindTrace:
		return extractTraceChannelResultPayload(message)
	default:
		return channelResultPayload{}, false, nil
	}
}

func extractSayChannelResultPayload(
	message storepkg.NetworkConversationMessage,
) (channelResultPayload, bool, error) {
	var body channelResultSayBody
	if len(bytes.TrimSpace(message.Body)) > 0 {
		if err := json.Unmarshal(message.Body, &body); err != nil {
			return channelResultPayload{}, false, fmt.Errorf("decode say result message %q: %w", message.MessageID, err)
		}
	}
	intent := firstNonEmpty(message.Intent, body.Intent)
	if intent != channelResultSignalIntent {
		return channelResultPayload{}, false, nil
	}
	text := firstNonEmpty(body.Text, message.Text, message.PreviewText)
	structured := cloneTrimmedRawMessage(body.Result)
	if len(structured) == 0 {
		structured = extractChannelResultTextJSON(text)
	}
	if len(structured) == 0 {
		structured = cloneObjectRawMessage(message.Body)
	}
	return channelResultPayload{structured: structured, text: text}, true, nil
}

func extractTraceChannelResultPayload(
	message storepkg.NetworkConversationMessage,
) (channelResultPayload, bool, error) {
	var body channelResultTraceBody
	if len(bytes.TrimSpace(message.Body)) == 0 {
		return channelResultPayload{}, false, nil
	}
	if err := json.Unmarshal(message.Body, &body); err != nil {
		return channelResultPayload{}, false, fmt.Errorf("decode trace result message %q: %w", message.MessageID, err)
	}
	if strings.TrimSpace(body.State) != storepkg.NetworkWorkStateCompleted {
		return channelResultPayload{}, false, nil
	}
	structured := cloneTrimmedRawMessage(body.Result)
	if len(structured) == 0 {
		return channelResultPayload{}, false, nil
	}
	text := firstNonEmpty(body.Message, message.Text, message.PreviewText)
	return channelResultPayload{structured: structured, text: text}, true, nil
}

func extractChannelResultTextJSON(text string) json.RawMessage {
	raw, err := extractJSONObject(text)
	if err != nil {
		return nil
	}
	return cloneRawMessage(raw)
}

func cloneTrimmedRawMessage(raw json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil
	}
	return cloneRawMessage(trimmed)
}

func cloneObjectRawMessage(raw json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil
	}
	var value map[string]any
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return nil
	}
	return cloneRawMessage(trimmed)
}
