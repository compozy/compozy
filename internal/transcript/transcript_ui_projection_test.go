package transcript

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	commandpkg "github.com/compozy/compozy/internal/command"
	"github.com/compozy/compozy/internal/store"
)

func TestClarificationEventProjectionPreservesTypedEvidence(t *testing.T) {
	t.Run("Should round-trip clarification evidence into the UI data payload", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`{
			"status":"resolved",
			"request":{"request_id":"req-clarify","question":"Which channel?"},
			"answer":{"choice":1,"text":"","fallback":false},
			"at":"2026-07-15T12:01:00Z"
		}`)
		content, err := MarshalAgentEvent(acp.AgentEvent{
			Type:      acp.EventTypeClarify,
			SessionID: "sess-clarify",
			TurnID:    "clarify:req-clarify",
			Timestamp: time.Date(2026, 7, 15, 12, 1, 0, 0, time.UTC),
			Raw:       raw,
		}.WithRequestID("req-clarify"))
		if err != nil {
			t.Fatalf("MarshalAgentEvent() error = %v", err)
		}
		decoded, err := UnmarshalAgentEvent(content)
		if err != nil {
			t.Fatalf("UnmarshalAgentEvent() error = %v", err)
		}
		payload := UIAgentEventPayloadFromEvent(decoded)
		if got, want := payload.Type, acp.EventTypeClarify; got != want {
			t.Fatalf("payload type = %q, want %q", got, want)
		}
		if got, want := payload.RequestID, "req-clarify"; got != want {
			t.Fatalf("payload request id = %q, want %q", got, want)
		}
		var gotRaw, wantRaw any
		if err := json.Unmarshal(payload.Raw, &gotRaw); err != nil {
			t.Fatalf("json.Unmarshal(payload raw) error = %v", err)
		}
		if err := json.Unmarshal(raw, &wantRaw); err != nil {
			t.Fatalf("json.Unmarshal(expected raw) error = %v", err)
		}
		if !reflect.DeepEqual(gotRaw, wantRaw) {
			t.Fatalf("payload raw = %#v, want %#v", gotRaw, wantRaw)
		}
	})
}

func TestToUIMessagesPreservesUserMessageIdentity(t *testing.T) {
	t.Run("Should project the durable user message identity into user metadata", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 7, 13, 23, 55, 0, 0, time.UTC)
		messageID := "user-message-identity"
		event := acp.AgentEvent{
			Type:      acp.EventTypeUserMessage,
			TurnID:    "turn-user-identity",
			Timestamp: timestamp,
			Text:      "Transformed provider input.",
		}.WithMessageID(messageID)
		authoredText := "Keep this message after reload."
		payload, err := MarshalPromptInputEvent(event, authoredText)
		if err != nil {
			t.Fatalf("MarshalAgentEvent() error = %v", err)
		}
		decoded, err := UnmarshalAgentEvent(payload)
		if err != nil {
			t.Fatalf("UnmarshalAgentEvent() error = %v", err)
		}
		if got, want := decoded.MessageIDValue(), messageID; got != want {
			t.Fatalf("MessageID = %q, want %q", got, want)
		}
		if got, want := decoded.Text, event.Text; got != want {
			t.Fatalf("effective Text = %q, want %q", got, want)
		}

		messages, err := ToUIMessages([]store.SessionEvent{{
			ID:        "ev-user-identity",
			SessionID: "sess-user-identity",
			TurnID:    event.TurnID,
			Sequence:  1,
			Type:      event.Type,
			Content:   payload,
			Timestamp: timestamp,
		}})
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if got, want := len(messages), 1; got != want {
			t.Fatalf("len(messages) = %d, want %d", got, want)
		}
		var metadata struct {
			TurnID    string `json:"turn_id"`
			MessageID string `json:"message_id"`
		}
		if err := json.Unmarshal(messages[0].Metadata, &metadata); err != nil {
			t.Fatalf("json.Unmarshal(metadata) error = %v", err)
		}
		if metadata.TurnID != event.TurnID || metadata.MessageID != messageID {
			t.Fatalf("metadata = %#v, want turn/message identity", metadata)
		}
		if got, want := messages[0].Parts[0].Text, authoredText; got != want {
			t.Fatalf("projected user text = %q, want exact authored text %q", got, want)
		}
	})

	t.Run("Should project authored skill markers from durable invocation metadata", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 8, 4, 15, 0, 0, 0, time.UTC)
		event := (acp.AgentEvent{
			Type:      acp.EventTypeUserMessage,
			TurnID:    "turn-skill-marker",
			Timestamp: timestamp,
			Text:      "provider input with injected instructions",
		}).WithSkillInvocations([]commandpkg.Invocation{{
			Ref: commandpkg.SkillRef{
				CommandID: "skill:extension:ops:review",
				Name:      "review",
				Source: commandpkg.Source{
					Kind: "extension", ID: "ops", Key: "ops", Scope: "workspace",
				},
			},
			Token: "/ops:review", Start: 7, End: 18,
		}})
		authoredText := "Please /ops:review this change"
		payload, err := MarshalPromptInputEvent(event, authoredText)
		if err != nil {
			t.Fatalf("MarshalPromptInputEvent() error = %v", err)
		}
		messages, err := ToUIMessages([]store.SessionEvent{{
			ID: "ev-skill-marker", SessionID: "sess-skill-marker", TurnID: event.TurnID,
			Sequence: 1, Type: event.Type, Content: payload, Timestamp: timestamp,
		}})
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if len(messages) != 1 || messages[0].Parts[0].Text != authoredText {
			t.Fatalf("messages = %#v, want exact authored text", messages)
		}
		var metadata struct {
			SkillInvocations []struct {
				CommandID string `json:"command_id"`
				Token     string `json:"token"`
				Label     string `json:"label"`
				Source    string `json:"source"`
				Start     int    `json:"start"`
				End       int    `json:"end"`
			} `json:"skill_invocations"`
		}
		if err := json.Unmarshal(messages[0].Metadata, &metadata); err != nil {
			t.Fatalf("json.Unmarshal(metadata) error = %v", err)
		}
		if len(metadata.SkillInvocations) != 1 ||
			metadata.SkillInvocations[0].CommandID != event.SkillInvocations()[0].Ref.CommandID ||
			metadata.SkillInvocations[0].Token != "/ops:review" ||
			metadata.SkillInvocations[0].Label != "review" ||
			metadata.SkillInvocations[0].Source != "extension:ops" ||
			metadata.SkillInvocations[0].Start != 7 ||
			metadata.SkillInvocations[0].End != 18 {
			t.Fatalf("skill invocation metadata = %#v", metadata.SkillInvocations)
		}
	})
}

func TestToUIMessagesProjectsMixedUserAttachments(t *testing.T) {
	t.Parallel()

	t.Run("Should project attachment file parts before authored text", func(t *testing.T) {
		t.Parallel()

		attachments := []acp.EventAttachment{
			{
				ID:       "att-image",
				Name:     "diagram.png",
				MIMEType: "image/png",
				Bytes:    2048,
				SHA256:   "sha-image",
				Kind:     "image",
				Width:    1280,
				Height:   720,
			},
			{
				ID:       "att-file",
				Name:     "notes.pdf",
				MIMEType: "application/pdf",
				Bytes:    4096,
				SHA256:   "sha-file",
				Kind:     "file",
			},
		}
		event := mustUIAgentSessionEvent(t, "ev-user-attachments", 1, time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC),
			(acp.AgentEvent{
				Type:      acp.EventTypeUserMessage,
				SessionID: "sess-attachments",
				TurnID:    "turn-attachments",
				Text:      "Review these files.",
			}).WithAttachments(attachments),
		)

		messages, err := ToUIMessages([]store.SessionEvent{event})
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if got, want := len(messages), 1; got != want {
			t.Fatalf("len(messages) = %d, want %d", got, want)
		}
		parts := messages[0].Parts
		if got, want := len(parts), 3; got != want {
			t.Fatalf("len(parts) = %d, want %d; parts=%#v", got, want, parts)
		}
		if got, want := parts[0], (UIMessagePart{
			Type: UIMessagePartTypeFile, MediaType: "image/png",
			URL: "compozy://session-attachments/att-image", Filename: "diagram.png",
		}); !reflect.DeepEqual(got, want) {
			t.Fatalf("parts[0] = %#v, want %#v", got, want)
		}
		if got, want := parts[1], (UIMessagePart{
			Type: UIMessagePartTypeFile, MediaType: "application/pdf",
			URL: "compozy://session-attachments/att-file", Filename: "notes.pdf",
		}); !reflect.DeepEqual(got, want) {
			t.Fatalf("parts[1] = %#v, want %#v", got, want)
		}
		if got, want := parts[2], (UIMessagePart{Type: uiPartText, Text: "Review these files.", State: uiPartStateDone}); !reflect.DeepEqual(
			got,
			want,
		) {
			t.Fatalf("parts[2] = %#v, want %#v", got, want)
		}

		var metadata struct {
			Attachments []acp.EventAttachment `json:"attachments"`
		}
		if err := json.Unmarshal(messages[0].Metadata, &metadata); err != nil {
			t.Fatalf("json.Unmarshal(metadata) error = %v", err)
		}
		if !slices.Equal(metadata.Attachments, attachments) {
			t.Fatalf("metadata attachments = %#v, want %#v", metadata.Attachments, attachments)
		}
		encoded, err := json.Marshal(messages[0])
		if err != nil {
			t.Fatalf("json.Marshal(materialized message) error = %v", err)
		}
		var reopened UIMessage
		if err := json.Unmarshal(encoded, &reopened); err != nil {
			t.Fatalf("json.Unmarshal(materialized message) error = %v", err)
		}
		if !reflect.DeepEqual(reopened, messages[0]) {
			t.Fatalf("reopened message = %#v, want %#v", reopened, messages[0])
		}
	})
}

func TestToUIMessagesProjectsImageOnlyUserAttachment(t *testing.T) {
	t.Parallel()

	t.Run("Should omit the text part for an image-only user message", func(t *testing.T) {
		t.Parallel()

		event := mustUIAgentSessionEvent(t, "ev-image-only", 1, time.Date(2026, 8, 14, 12, 1, 0, 0, time.UTC),
			(acp.AgentEvent{Type: acp.EventTypeUserMessage, SessionID: "sess-image-only", TurnID: "turn-image-only"}).
				WithAttachments([]acp.EventAttachment{{ID: "att-only", Name: "only.png", MIMEType: "image/png"}}),
		)

		messages, err := ToUIMessages([]store.SessionEvent{event})
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		want := []UIMessagePart{{
			Type: UIMessagePartTypeFile, MediaType: "image/png",
			URL: "compozy://session-attachments/att-only", Filename: "only.png",
		}}
		if len(messages) != 1 || !reflect.DeepEqual(messages[0].Parts, want) {
			t.Fatalf("messages = %#v, want image-only parts %#v", messages, want)
		}
	})
}

func TestToUIMessagesProjectsTextOnlyUserMessage(t *testing.T) {
	t.Parallel()

	t.Run("Should leave text-only user messages unchanged", func(t *testing.T) {
		t.Parallel()

		event := mustUIAgentSessionEvent(
			t,
			"ev-text-only",
			1,
			time.Date(2026, 8, 14, 12, 2, 0, 0, time.UTC),
			acp.AgentEvent{
				Type:      acp.EventTypeUserMessage,
				SessionID: "sess-text-only",
				TurnID:    "turn-text-only",
				Text:      "Text only.",
			},
		)
		messages, err := ToUIMessages([]store.SessionEvent{event})
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		want := []UIMessagePart{{Type: uiPartText, Text: "Text only.", State: uiPartStateDone}}
		if len(messages) != 1 || !reflect.DeepEqual(messages[0].Parts, want) {
			t.Fatalf("messages = %#v, want unchanged text-only parts %#v", messages, want)
		}
	})
}

func TestToUIMessagesPermissionDataParts(t *testing.T) {
	t.Run("ShouldReplacePendingPermissionWithFinalDecision", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
		events := []store.SessionEvent{
			mustPermissionSessionEvent(t, "ev-pending", 1, timestamp, ""),
			mustPermissionSessionEvent(t, "ev-final", 2, timestamp.Add(time.Second), "allow-once"),
		}

		messages, err := ToUIMessages(events)
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if got, want := len(messages), 1; got != want {
			t.Fatalf("len(messages) = %d, want %d", got, want)
		}
		if got, want := len(messages[0].Parts), 1; got != want {
			t.Fatalf("len(parts) = %d, want %d; parts=%#v", got, want, messages[0].Parts)
		}

		part := messages[0].Parts[0]
		if got, want := part.Type, uiPartDataPermission; got != want {
			t.Fatalf("part.Type = %q, want %q", got, want)
		}
		if got, want := part.ID, "req-permission"; got != want {
			t.Fatalf("part.ID = %q, want %q", got, want)
		}

		var payload UIAgentEventPayload
		if err := json.Unmarshal(part.Data, &payload); err != nil {
			t.Fatalf("json.Unmarshal(part.Data) error = %v", err)
		}
		if got, want := payload.Decision, "allow-once"; got != want {
			t.Fatalf("payload.Decision = %q, want %q", got, want)
		}
	})

	t.Run("ShouldPreservePendingPermissionWithoutDecision", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
		events := []store.SessionEvent{
			mustPermissionSessionEvent(t, "ev-pending", 1, timestamp, ""),
		}

		messages, err := ToUIMessages(events)
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if got, want := len(messages), 1; got != want {
			t.Fatalf("len(messages) = %d, want %d", got, want)
		}
		if got, want := len(messages[0].Parts), 1; got != want {
			t.Fatalf("len(parts) = %d, want %d; parts=%#v", got, want, messages[0].Parts)
		}

		part := messages[0].Parts[0]
		if got, want := part.ID, "req-permission"; got != want {
			t.Fatalf("part.ID = %q, want %q", got, want)
		}

		var payload UIAgentEventPayload
		if err := json.Unmarshal(part.Data, &payload); err != nil {
			t.Fatalf("json.Unmarshal(part.Data) error = %v", err)
		}
		if payload.Decision != "" {
			t.Fatalf("payload.Decision = %q, want empty", payload.Decision)
		}
	})

	t.Run("ShouldPreservePermissionOptionsForReplay", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
		content, err := MarshalAgentEvent(acp.AgentEvent{
			Type:      acp.EventTypePermission,
			SessionID: "sess-permission",
			TurnID:    "turn-permission",
			Timestamp: timestamp,
			Title:     "Bash",
			Action:    "session/request_permission",
			Resource:  "Bash",
			Raw: json.RawMessage(`{
				"request_id":"req-permission-options",
				"options":[
					{"decision":"allow-once","option_id":"allow-once","kind":"allow_once"},
					{"decision":"reject-once","option_id":"reject-once","kind":"reject_once"}
				],
				"tool_input":{"command":"touch blocked.txt"}
			}`),
		}.WithRequestID("req-permission-options"))
		if err != nil {
			t.Fatalf("MarshalAgentEvent() error = %v", err)
		}

		event := store.SessionEvent{
			ID:        "ev-permission-options",
			SessionID: "sess-permission",
			TurnID:    "turn-permission",
			Sequence:  1,
			Type:      acp.EventTypePermission,
			Content:   content,
			Timestamp: timestamp,
		}
		messages, err := ToUIMessages([]store.SessionEvent{event})
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if got, want := len(messages), 1; got != want {
			t.Fatalf("len(messages) = %d, want %d", got, want)
		}
		if got, want := len(messages[0].Parts), 1; got != want {
			t.Fatalf("len(parts) = %d, want %d; parts=%#v", got, want, messages[0].Parts)
		}

		var payload UIAgentEventPayload
		if err := json.Unmarshal(messages[0].Parts[0].Data, &payload); err != nil {
			t.Fatalf("json.Unmarshal(part.Data) error = %v", err)
		}
		var raw struct {
			Options []struct {
				Decision string `json:"decision"`
			} `json:"options"`
			ToolInput struct {
				Command string `json:"command"`
			} `json:"tool_input"`
		}
		if err := json.Unmarshal(payload.Raw, &raw); err != nil {
			t.Fatalf("json.Unmarshal(payload.Raw) error = %v", err)
		}
		if got, want := len(raw.Options), 2; got != want {
			t.Fatalf("len(raw.Options) = %d, want %d", got, want)
		}
		if got, want := raw.Options[0].Decision, "allow-once"; got != want {
			t.Fatalf("raw.Options[0].Decision = %q, want %q", got, want)
		}
		if got, want := raw.Options[1].Decision, "reject-once"; got != want {
			t.Fatalf("raw.Options[1].Decision = %q, want %q", got, want)
		}
		if got, want := raw.ToolInput.Command, "touch blocked.txt"; got != want {
			t.Fatalf("raw.ToolInput.Command = %q, want %q", got, want)
		}
	})
}

func TestToUIMessagesOrderedAssistantParts(t *testing.T) {
	t.Run("ShouldPreserveFatalPromptErrorAsDataPart", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 5, 14, 15, 32, 0, 0, time.UTC)
		errorText := `{"code":-32603,"message":"Internal error","data":{"error":"peer disconnected before response"}}`
		events := []store.SessionEvent{
			mustUIAgentSessionEvent(t, "ev-text", 1, timestamp, acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: "sess-failed",
				TurnID:    "turn-failed",
				Timestamp: timestamp,
				Text:      "partial response",
			}),
			mustUIAgentSessionEvent(t, "ev-error", 2, timestamp.Add(time.Second), acp.AgentEvent{
				Type:      acp.EventTypeError,
				SessionID: "sess-failed",
				TurnID:    "turn-failed",
				Timestamp: timestamp.Add(time.Second),
				Error:     errorText,
				Failure: &store.SessionFailure{
					Kind:    store.FailureProcess,
					Summary: "peer disconnected before response",
				},
			}),
		}

		messages, err := ToUIMessages(events)
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if got, want := len(messages), 1; got != want {
			t.Fatalf("len(messages) = %d, want %d; messages=%#v", got, want, messages)
		}
		if got, want := len(messages[0].Parts), 2; got != want {
			t.Fatalf("len(messages[0].Parts) = %d, want %d; parts=%#v", got, want, messages[0].Parts)
		}
		if got, want := messages[0].Parts[0].Type, uiPartText; got != want {
			t.Fatalf("parts[0].Type = %q, want %q", got, want)
		}
		if got, want := messages[0].Parts[0].State, uiPartStateDone; got != want {
			t.Fatalf("parts[0].State = %q, want %q", got, want)
		}

		errorPart := messages[0].Parts[1]
		if got, want := errorPart.Type, uiPartDataEvent; got != want {
			t.Fatalf("parts[1].Type = %q, want %q", got, want)
		}

		var payload UIAgentEventPayload
		if err := json.Unmarshal(errorPart.Data, &payload); err != nil {
			t.Fatalf("json.Unmarshal(errorPart.Data) error = %v", err)
		}
		if got, want := payload.Type, acp.EventTypeError; got != want {
			t.Fatalf("payload.Type = %q, want %q", got, want)
		}
		if got, want := payload.Error, errorText; got != want {
			t.Fatalf("payload.Error = %q, want %q", got, want)
		}
		if payload.Failure == nil {
			t.Fatal("payload.Failure = nil, want process failure")
		}
		if got, want := payload.Failure.Kind, store.FailureProcess; got != want {
			t.Fatalf("payload.Failure.Kind = %q, want %q", got, want)
		}
	})

	t.Run("ShouldPreserveTextToolTextOrderInsideOneAssistantMessage", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 5, 13, 10, 0, 0, 0, time.UTC)
		events := []store.SessionEvent{
			mustUIAgentSessionEvent(t, "ev-text-1", 1, timestamp, acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: "sess-mixed",
				TurnID:    "turn-mixed",
				Timestamp: timestamp,
				Text:      "text 1",
			}),
			mustUIAgentSessionEvent(t, "ev-tool-2", 2, timestamp.Add(time.Second), acp.AgentEvent{
				Type:       acp.EventTypeToolCall,
				SessionID:  "sess-mixed",
				TurnID:     "turn-mixed",
				Timestamp:  timestamp.Add(time.Second),
				Title:      "Bash",
				ToolCallID: "tool-2",
				Raw:        json.RawMessage("{\"rawInput\":{\"command\":\"pwd\"}}"),
			}),
			mustUIAgentSessionEvent(t, "ev-tool-3", 3, timestamp.Add(2*time.Second), acp.AgentEvent{
				Type:       acp.EventTypeToolCall,
				SessionID:  "sess-mixed",
				TurnID:     "turn-mixed",
				Timestamp:  timestamp.Add(2 * time.Second),
				Title:      "Read",
				ToolCallID: "tool-3",
				Raw:        json.RawMessage("{\"rawInput\":{\"file_path\":\"README.md\"}}"),
			}),
			mustUIAgentSessionEvent(t, "ev-text-4", 4, timestamp.Add(3*time.Second), acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: "sess-mixed",
				TurnID:    "turn-mixed",
				Timestamp: timestamp.Add(3 * time.Second),
				Text:      "text 4",
			}),
			mustUIAgentSessionEvent(t, "ev-text-5", 5, timestamp.Add(4*time.Second), acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: "sess-mixed",
				TurnID:    "turn-mixed",
				Timestamp: timestamp.Add(4 * time.Second),
				Text:      "text 5",
			}),
			mustUIAgentSessionEvent(t, "ev-done", 6, timestamp.Add(5*time.Second), acp.AgentEvent{
				Type:             acp.EventTypeDone,
				SessionID:        "sess-mixed",
				TurnID:           "turn-mixed",
				Timestamp:        timestamp.Add(5 * time.Second),
				StopReason:       string(acp.PromptStopReasonEndTurn),
				PromptStopReason: acp.PromptStopReasonEndTurn,
			}),
		}

		messages, err := ToUIMessages(events)
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if got, want := len(messages), 1; got != want {
			t.Fatalf("len(messages) = %d, want %d; messages=%#v", got, want, messages)
		}

		got := uiVisiblePartSignatures(messages[0].Parts)
		want := []string{
			"text:turn-mixed-text-1:text 1:done",
			"tool-Bash:tool-2:input-available",
			"tool-Read:tool-3:input-available",
			"text:turn-mixed-text-2:text 4text 5:done",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("visible part signatures = %#v, want %#v; parts=%#v", got, want, messages[0].Parts)
		}
	})

	t.Run("ShouldPreserveReasoningAsASeparateOrderedPart", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 5, 13, 11, 0, 0, 0, time.UTC)
		events := []store.SessionEvent{
			mustUIAgentSessionEvent(t, "ev-text-1", 1, timestamp, acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: "sess-reasoning",
				TurnID:    "turn-reasoning",
				Timestamp: timestamp,
				Text:      "visible",
			}),
			mustUIAgentSessionEvent(t, "ev-thought-1", 2, timestamp.Add(time.Second), acp.AgentEvent{
				Type:      acp.EventTypeThought,
				SessionID: "sess-reasoning",
				TurnID:    "turn-reasoning",
				Timestamp: timestamp.Add(time.Second),
				Text:      "checking",
			}),
			mustUIAgentSessionEvent(t, "ev-text-2", 3, timestamp.Add(2*time.Second), acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: "sess-reasoning",
				TurnID:    "turn-reasoning",
				Timestamp: timestamp.Add(2 * time.Second),
				Text:      "answer",
			}),
			mustUIAgentSessionEvent(t, "ev-done", 4, timestamp.Add(3*time.Second), acp.AgentEvent{
				Type:             acp.EventTypeDone,
				SessionID:        "sess-reasoning",
				TurnID:           "turn-reasoning",
				Timestamp:        timestamp.Add(3 * time.Second),
				StopReason:       string(acp.PromptStopReasonEndTurn),
				PromptStopReason: acp.PromptStopReasonEndTurn,
			}),
		}

		messages, err := ToUIMessages(events)
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if got, want := len(messages), 1; got != want {
			t.Fatalf("len(messages) = %d, want %d; messages=%#v", got, want, messages)
		}

		got := uiVisiblePartSignatures(messages[0].Parts)
		want := []string{
			"text:turn-reasoning-text-1:visible:done",
			"reasoning:turn-reasoning-reasoning-1:checking:done",
			"text:turn-reasoning-text-2:answer:done",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("visible part signatures = %#v, want %#v; parts=%#v", got, want, messages[0].Parts)
		}
	})
}
