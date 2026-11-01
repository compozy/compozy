package transcript

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	commandpkg "github.com/compozy/compozy/internal/command"
	"github.com/compozy/compozy/internal/mention"
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
			Timestamp string `json:"timestamp"`
		}
		if err := json.Unmarshal(messages[0].Metadata, &metadata); err != nil {
			t.Fatalf("json.Unmarshal(metadata) error = %v", err)
		}
		if metadata.TurnID != event.TurnID || metadata.MessageID != messageID ||
			metadata.Timestamp != timestamp.Format(time.RFC3339Nano) {
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

	t.Run("Should project verified mentions with relative targets onto user metadata", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
		authoredText := "Review @internal/acp/client_prompt.go and compare with @session:sess-9f8e7d"
		event := (acp.AgentEvent{
			Type: acp.EventTypeUserMessage, TurnID: "turn-mentions", Timestamp: timestamp, Text: authoredText,
		}).WithMentions([]mention.Mention{
			{
				Kind: mention.KindFile, Token: "@internal/acp/client_prompt.go",
				Target: "internal/acp/client_prompt.go", Label: "client_prompt.go", Start: 7, End: 37,
			},
			{
				Kind: mention.KindSession, Token: "@session:sess-9f8e7d", Target: "sess-9f8e7d",
				Label: "Fix login bug", Start: 55, End: 75,
			},
		})
		payload, err := MarshalPromptInputEvent(event, authoredText)
		if err != nil {
			t.Fatalf("MarshalPromptInputEvent() error = %v", err)
		}
		messages, err := ToUIMessages([]store.SessionEvent{{
			ID: "ev-mentions", SessionID: "sess-mentions", TurnID: event.TurnID,
			Sequence: 1, Type: event.Type, Content: payload, Timestamp: timestamp,
		}})
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if len(messages) != 1 {
			t.Fatalf("messages = %#v, want one user message", messages)
		}
		var metadata struct {
			Mentions json.RawMessage `json:"mentions"`
		}
		if err := json.Unmarshal(messages[0].Metadata, &metadata); err != nil {
			t.Fatalf("json.Unmarshal(metadata) error = %v", err)
		}
		const want = `[` +
			`{"kind":"file","token":"@internal/acp/client_prompt.go","target":"internal/acp/client_prompt.go",` +
			`"label":"client_prompt.go","start":7,"end":37},` +
			`{"kind":"session","token":"@session:sess-9f8e7d","target":"sess-9f8e7d",` +
			`"label":"Fix login bug","start":55,"end":75}]`
		if string(metadata.Mentions) != want {
			t.Fatalf("metadata.mentions = %s, want %s", metadata.Mentions, want)
		}
	})

	t.Run("Should omit mentions metadata for input events stored without mentions", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
		legacy := `{"type":"user_message","turn_id":"turn-legacy","text":"@notes.md hello"}`
		messages, err := ToUIMessages([]store.SessionEvent{{
			ID: "ev-legacy", SessionID: "sess-legacy", TurnID: "turn-legacy",
			Sequence: 1, Type: acp.EventTypeUserMessage, Content: legacy, Timestamp: timestamp,
		}})
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if len(messages) != 1 || messages[0].Parts[len(messages[0].Parts)-1].Text != "@notes.md hello" {
			t.Fatalf("messages = %#v, want the legacy text unchanged", messages)
		}
		if strings.Contains(string(messages[0].Metadata), `"mentions"`) {
			t.Fatalf("metadata = %s, want no mentions key", messages[0].Metadata)
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

// UT-039: existing legacy fixture projections retain their ordered parts unchanged.
func TestToUIMessagesOrderedAssistantParts(t *testing.T) {
	t.Run("Should retain legacy compaction calls as ordinary tool rows", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
		event := acp.AgentEvent{
			Type:       acp.EventTypeToolCall,
			SessionID:  "sess-legacy-compaction",
			TurnID:     "turn-legacy-compaction",
			Timestamp:  timestamp,
			Title:      "Compact conversation",
			ToolCallID: "legacy-compact-1",
			Raw: json.RawMessage(
				`{"sessionUpdate":"tool_call","toolCallId":"legacy-compact-1","title":"Compact conversation","kind":"think","rawInput":{}}`,
			),
		}
		messages, err := ToUIMessages([]store.SessionEvent{
			mustUIAgentSessionEvent(t, "ev-legacy-compaction", 1, timestamp, event),
		})
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if len(messages) != 1 || messages[0].Role != UIRoleAssistant {
			t.Fatalf("messages = %#v, want one assistant message", messages)
		}
		if got, want := uiVisiblePartSignatures(
			messages[0].Parts,
		), []string{
			"tool-Compact conversation:legacy-compact-1:input-streaming",
		}; !reflect.DeepEqual(
			got,
			want,
		) {
			t.Fatalf("visible part signatures = %#v, want %#v", got, want)
		}
		part := messages[0].Parts[0]
		if part.Type != "tool-Compact conversation" || part.ToolCallID != event.ToolCallID ||
			part.Title != event.Title || part.State != uiToolStateStreaming {
			t.Fatalf("part = %#v, want streaming legacy compaction tool row", part)
		}
	})

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

	t.Run("Should name the turn of a text-only continuation split by a steer marker", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 10, 23, 17, 3, 0, 0, time.UTC)
		marker, err := NewMarker(MarkerPromptSteered, "Subagent finished.", timestamp.Add(time.Second),
			map[string]any{"kind": "subagent_wake", "message_id": "wake_01M5MV5SVJGJVBC5P1278DP0KA"})
		if err != nil {
			t.Fatalf("NewMarker() error = %v", err)
		}
		markerEvent, err := marker.AgentEvent("sess-split", "turn-split")
		if err != nil {
			t.Fatalf("AgentEvent() error = %v", err)
		}
		events := []store.SessionEvent{
			mustUIAgentSessionEvent(t, "ev-text-1", 1, timestamp, acp.AgentEvent{
				Type: acp.EventTypeAgentMessage, SessionID: "sess-split", TurnID: "turn-split",
				Timestamp: timestamp, Text: "before the steer",
			}),
			mustUIAgentSessionEvent(t, "ev-marker", 2, markerEvent.Timestamp, markerEvent),
			mustUIAgentSessionEvent(t, "ev-text-3", 3, timestamp.Add(2*time.Second), acp.AgentEvent{
				Type: acp.EventTypeAgentMessage, SessionID: "sess-split", TurnID: "turn-split",
				Timestamp: timestamp.Add(2 * time.Second), Text: "after the steer",
			}),
		}

		messages, err := ToUIMessages(events)
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if got, want := len(messages), 3; got != want {
			t.Fatalf("len(messages) = %d, want %d; messages=%#v", got, want, messages)
		}
		if messages[0].ID != "turn-split" || len(messages[0].Metadata) != 0 {
			t.Fatalf(
				"first chunk = %q metadata %s, want turn-split without metadata",
				messages[0].ID,
				messages[0].Metadata,
			)
		}
		continuation := messages[2]
		if continuation.ID == "turn-split" {
			t.Fatalf("continuation id = %q, want a distinct projected id", continuation.ID)
		}
		var metadata struct {
			TurnID string `json:"turn_id"`
		}
		if err := json.Unmarshal(continuation.Metadata, &metadata); err != nil || metadata.TurnID != "turn-split" {
			t.Fatalf("continuation metadata = %s (err %v), want turn_id turn-split", continuation.Metadata, err)
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

	t.Run("Should keep an identity-less tool update with the turn that announced the call", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 10, 18, 3, 44, 0, 0, time.UTC)
		rawEvent := func(id string, sequence int64, turnID string, eventType string, content string) store.SessionEvent {
			return store.SessionEvent{
				ID:        id,
				SessionID: "sess-goal",
				TurnID:    turnID,
				Sequence:  sequence,
				Type:      eventType,
				Content:   content,
				Timestamp: timestamp.Add(time.Duration(sequence) * time.Second),
			}
		}
		events := []store.SessionEvent{
			rawEvent("ev-call", 1, "goal-1", acp.EventTypeToolCall, `{"schema":"compozy.session.event.v1",`+
				`"type":"tool_call","turn_id":"goal-1","status":"in_progress","title":"make gate",`+
				`"tool_name":"make gate","tool_kind":"execute","tool_call_id":"exec-1",`+
				`"tool_input":{"command":"make gate"}}`),
			mustUIAgentSessionEvent(t, "ev-done-1", 2, timestamp.Add(2*time.Second), acp.AgentEvent{
				Type:             acp.EventTypeDone,
				SessionID:        "sess-goal",
				TurnID:           "goal-1",
				StopReason:       string(acp.PromptStopReasonEndTurn),
				PromptStopReason: acp.PromptStopReasonEndTurn,
			}),
			mustUIAgentSessionEvent(t, "ev-text-2", 3, timestamp.Add(3*time.Second), acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: "sess-goal",
				TurnID:    "goal-2",
				Text:      "still waiting on the gate",
			}),
			rawEvent("ev-call-update", 4, "goal-2", acp.EventTypeToolCall, `{"schema":"compozy.session.event.v1",`+
				`"type":"tool_call","turn_id":"goal-2","tool_call_id":"exec-1"}`),
			rawEvent("ev-result", 5, "goal-2", acp.EventTypeToolResult, `{"schema":"compozy.session.event.v1",`+
				`"type":"tool_result","turn_id":"goal-2","status":"failed","tool_call_id":"exec-1",`+
				`"tool_result":{"raw_output":{"exit_code":2}},"tool_error":true}`),
		}

		messages, err := ToUIMessages(events)
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if got, want := len(messages), 2; got != want {
			t.Fatalf("len(messages) = %d, want %d; messages=%#v", got, want, messages)
		}
		gotTurns := [][]string{
			uiVisiblePartSignatures(messages[0].Parts),
			uiVisiblePartSignatures(messages[1].Parts),
		}
		wantTurns := [][]string{
			{"tool-make gate:exec-1:output-error"},
			{"text:goal-2-text-1:still waiting on the gate:streaming"},
		}
		if !reflect.DeepEqual(gotTurns, wantTurns) {
			t.Fatalf("visible part signatures = %#v, want %#v", gotTurns, wantTurns)
		}
	})

	t.Run("Should name and describe a dynamic tool failure whose provider reports neither", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 10, 18, 4, 0, 0, 0, time.UTC)
		events := []store.SessionEvent{{
			ID:        "ev-orphan-result",
			SessionID: "sess-orphan",
			TurnID:    "turn-orphan",
			Sequence:  1,
			Type:      acp.EventTypeToolResult,
			Content: `{"schema":"compozy.session.event.v1","type":"tool_result","turn_id":"turn-orphan",` +
				`"status":"failed","tool_call_id":"exec-orphan","tool_result":{"raw_output":{"exit_code":2}},` +
				`"tool_error":true}`,
			Timestamp: timestamp,
		}}

		messages, err := ToUIMessages(events)
		if err != nil {
			t.Fatalf("ToUIMessages() error = %v", err)
		}
		if got, want := len(messages), 1; got != want {
			t.Fatalf("len(messages) = %d, want %d; messages=%#v", got, want, messages)
		}
		var tool *UIMessagePart
		for index := range messages[0].Parts {
			if messages[0].Parts[index].Type == uiPartDynamicTool {
				tool = &messages[0].Parts[index]
			}
		}
		if tool == nil {
			t.Fatalf("parts = %#v, want one dynamic-tool part", messages[0].Parts)
		}
		if tool.ToolName != uiFallbackToolName || tool.State != uiToolStateError ||
			tool.ErrorText != uiFallbackToolErrorText {
			t.Fatalf(
				"dynamic tool = %#v, want toolName %q in state %q with errorText %q",
				tool, uiFallbackToolName, uiToolStateError, uiFallbackToolErrorText,
			)
		}
	})
}

// Invariant: snapshots with one compaction ID form one corrected item in the canonical UI projection.
func TestCompactionProjection(t *testing.T) {
	t.Parallel()
	t.Run("Should fold duplicate and corrective snapshots [UT-030]", func(t *testing.T) {
		t.Parallel()
		at := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
		observations := []acp.CompactionObservation{
			{CompactionID: "c1", Status: "in_progress"},
			{CompactionID: "c1", Status: "completed", Summary: strings.Repeat("a", 20*1024), Terminal: true},
			{CompactionID: "c1", Status: "completed", Summary: strings.Repeat("a", 20*1024)},
			{CompactionID: "c1", Status: "failed", Summary: strings.Repeat("a", 20*1024), Error: "corrected"},
		}
		events := make([]store.SessionEvent, 0, len(observations))
		for i, observation := range observations {
			events = append(
				events,
				mustUIAgentSessionEvent(
					t,
					fmt.Sprintf("c-%d", i),
					int64(i+1),
					at.Add(time.Duration(i)*time.Second),
					acp.AgentEvent{
						Type:       acp.EventTypeCompaction,
						SessionID:  "s1",
						TurnID:     "t1",
						Timestamp:  at.Add(time.Duration(i) * time.Second),
						Compaction: &observation,
					},
				),
			)
		}
		messages, err := ToUIMessages(events)
		if err != nil {
			t.Fatal(err)
		}
		if len(messages) != 1 || len(messages[0].Parts) != 1 {
			t.Fatalf("messages = %#v", messages)
		}
		var item CompactionItem
		if err := json.Unmarshal(messages[0].Parts[0].Data, &item); err != nil {
			t.Fatal(err)
		}
		if item.Kind != "compaction" || item.CompactionID != "c1" || item.Status != "failed" ||
			item.Error != "corrected" ||
			len(item.Summary) != 16*1024 ||
			!strings.HasSuffix(item.Summary, " [summary truncated]") ||
			item.EndedAt == nil ||
			!item.EndedAt.Equal(at.Add(time.Second)) {
			t.Fatalf("item = %#v", item)
		}
	})
}

// Invariant: a report is one data entry that leaves the running assistant message open.
func TestReportProjection(t *testing.T) {
	t.Parallel()
	t.Run("Should project a report beside a running turn", func(t *testing.T) {
		t.Parallel()
		at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
		turn := func(id string, seq int64, text string) store.SessionEvent {
			return mustUIAgentSessionEvent(t, id, seq, at, acp.AgentEvent{
				Type: acp.EventTypeAgentMessage, SessionID: "s1", TurnID: "t1", Timestamp: at, Text: text,
			})
		}
		messages, err := ToUIMessages([]store.SessionEvent{
			turn("a1", 1, "Hello "),
			{
				ID: "report:arpt-1", SessionID: "s1", TurnID: "report:arpt-1", Sequence: 2, Timestamp: at,
				Type: ReportEventType, Content: `{"report_id":"arpt-1","instance_id":"ainst-1","kind":"proposal","payload":{"proposals":[{"title":"Unblock","body":"Run 12"}]},"created_at":"2026-11-01T12:00:00Z"}`,
			},
			turn("a2", 3, "again"),
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(messages) != 2 {
			t.Fatalf("messages = %#v", messages)
		}
		report := messages[1]
		if report.ID != "report:arpt-1" || report.Role != UIRoleSystem || len(report.Parts) != 1 ||
			report.Parts[0].Type != "data-compozy-report" || report.Parts[0].ID != "arpt-1" {
			t.Fatalf("report = %#v", report)
		}
		var data struct {
			Kind    string `json:"kind"`
			Payload struct {
				Proposals []struct {
					Title string `json:"title"`
				} `json:"proposals"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(report.Parts[0].Data, &data); err != nil {
			t.Fatal(err)
		}
		if data.Kind != "proposal" || len(data.Payload.Proposals) != 1 || data.Payload.Proposals[0].Title != "Unblock" {
			t.Fatalf("data = %#v", data)
		}
		if got := messages[0].Parts[0].Text; got != "Hello again" {
			t.Fatalf("assistant text = %q", got)
		}
	})
	t.Run("Should leave an undecodable report out", func(t *testing.T) {
		t.Parallel()
		messages, err := ToUIMessages([]store.SessionEvent{{ID: "report:x", SessionID: "s1", Sequence: 1, Type: ReportEventType, Content: "{"}})
		if err != nil {
			t.Fatal(err)
		}
		if len(messages) != 0 {
			t.Fatalf("messages = %#v", messages)
		}
	})
}

func TestSubagentUIProjection(t *testing.T) {
	t.Run("Should project hosted delegate names from both metadata surfaces", func(t *testing.T) {
		t.Parallel()
		for _, name := range []string{"compozy__subagent_delegate", "mcp__compozy-hosted-tools__compozy__subagent_delegate", "mcp.compozy-hosted-tools.compozy__subagent_delegate", "compozy-hosted-tools.compozy__subagent_delegate"} {
			for _, providerMetadata := range []bool{false, true} {
				event := acp.AgentEvent{
					Type:       acp.EventTypeToolResult,
					SessionID:  "parent",
					TurnID:     "turn",
					ToolCallID: "call",
					Raw:        json.RawMessage(`{"rawOutput":{"subagent_id":"sub-delegated"}}`),
				}
				if providerMetadata {
					event = event.WithProviderToolMetadata("", name, "completed")
				} else {
					event = event.WithTool(name, nil, false)
				}
				content, err := MarshalAgentEvent(event)
				if err != nil {
					t.Fatal(err)
				}
				messages, err := ToUIMessages(
					[]store.SessionEvent{
						{
							ID:        "event",
							SessionID: "parent",
							TurnID:    "turn",
							Sequence:  1,
							Type:      event.Type,
							Content:   content,
						},
					},
				)
				if err != nil {
					t.Fatal(err)
				}
				cards := 0
				for _, message := range messages {
					for _, part := range message.Parts {
						if part.Type == "data-compozy-subagent" {
							cards++
						}
					}
				}
				if cards != 1 {
					t.Fatalf("%s (provider=%v): cards=%d", name, providerMetadata, cards)
				}
			}
		}
	})
	t.Run(
		"Should replace a generic native title with the streamed description without guessing its provider",
		func(t *testing.T) {
			t.Parallel()
			var rows []store.SessionEvent
			for i, input := range []string{`{}`, `{"description":"Survey BRIEF.md risks"}`} {
				event := acp.AgentEvent{
					Type:       acp.EventTypeToolCall,
					SessionID:  "parent",
					TurnID:     "turn",
					ToolCallID: "native",
					Title:      "Task",
				}.
					WithTool("Agent", json.RawMessage(input), false).
					WithProviderToolMetadata("", "Agent", "in_progress")
				content, err := MarshalAgentEvent(event)
				if err != nil {
					t.Fatal(err)
				}
				rows = append(
					rows,
					store.SessionEvent{
						ID:        fmt.Sprintf("event-%d", i),
						SessionID: "parent",
						TurnID:    "turn",
						Sequence:  int64(i + 1),
						Type:      event.Type,
						Content:   content,
					},
				)
			}
			messages, err := ToUIMessages(rows)
			if err != nil {
				t.Fatal(err)
			}
			cards := 0
			for _, message := range messages {
				for _, part := range message.Parts {
					if part.Type != "data-compozy-subagent" {
						continue
					}
					cards++
					var payload UISubagentPayload
					if err := json.Unmarshal(part.Data, &payload); err != nil {
						t.Fatal(err)
					}
					if payload.Title != "Survey BRIEF.md risks" || payload.RuntimeProvider != "" {
						t.Fatalf("payload=%#v", payload)
					}
				}
			}
			if cards != 1 {
				t.Fatalf("cards=%d", cards)
			}
		},
	)
	// Invariant: native task observations drive subagent rows only; the transcript projection is unchanged by them.
	t.Run("Should not project native task observations as transcript parts", func(t *testing.T) {
		t.Parallel()
		project := func(withTask bool) []UIMessage {
			t.Helper()
			events := []acp.AgentEvent{
				acp.AgentEvent{Type: acp.EventTypeToolCall, ToolCallID: "native", Title: "Review"}.
					WithProviderToolMetadata("", "Agent", "in_progress"),
			}
			if withTask {
				events = append(events, acp.AgentEvent{
					Type: acp.EventTypeNativeTask, ToolCallID: "native",
				}.WithNativeTask(&acp.NativeTaskObservation{
					Kind: acp.NativeTaskKindState, TaskID: "agent-a", ToolCallID: "native",
					Status: acp.NativeTaskStatusCompleted, Summary: new("done"),
				}))
			}
			events = append(events, acp.AgentEvent{Type: acp.EventTypeAgentMessage, Text: "After"})
			var stored []store.SessionEvent
			for i, event := range events {
				event.SessionID, event.TurnID = "parent", "turn"
				content, err := MarshalAgentEvent(event)
				if err != nil {
					t.Fatal(err)
				}
				stored = append(stored, store.SessionEvent{
					ID: fmt.Sprintf("event-%d", i), SessionID: "parent", TurnID: "turn",
					Sequence: int64(i + 1), Type: event.Type, Content: content,
				})
			}
			messages, err := ToUIMessages(stored)
			if err != nil {
				t.Fatal(err)
			}
			return messages
		}
		without, with := project(false), project(true)
		if len(with) != len(without) || len(with) != 1 || len(with[0].Parts) != len(without[0].Parts) {
			t.Fatalf("projection with task = %#v, without = %#v", with, without)
		}
		for _, part := range with[0].Parts {
			if bytes.Contains(part.Data, []byte(acp.EventTypeNativeTask)) {
				t.Fatalf("native task leaked into part %#v", part)
			}
		}
	})
	// UT-038: tool/text attribution does not merge child chunks into parent chunks.
	t.Run("Should project native and delegated cards at their transcript positions", func(t *testing.T) {
		t.Parallel()
		events := []acp.AgentEvent{
			{Type: acp.EventTypeAgentMessage, Text: "Before"},
			acp.AgentEvent{
				Type:       acp.EventTypeToolCall,
				ToolCallID: "native",
				Title:      "Review",
			}.WithProviderToolMetadata(
				"",
				"Agent",
				"pending",
			),
			acp.AgentEvent{Type: acp.EventTypeAgentMessage, Text: "Inner"}.WithProviderToolMetadata("native", "", ""),
			{Type: acp.EventTypeAgentMessage, Text: "After"},
			acp.AgentEvent{
				Type:       acp.EventTypeToolCall,
				ToolCallID: "inner-read",
				Title:      "Read",
			}.WithProviderToolMetadata(
				"native",
				"Read",
				"pending",
			),
			{
				Type:       acp.EventTypeToolResult,
				ToolCallID: "delegate",
				Raw:        json.RawMessage(`{"rawOutput":{"subagent_id":"sub-delegated"}}`),
			},
			{
				Type:       acp.EventTypeToolResult,
				ToolCallID: "failed",
				Raw:        json.RawMessage(`{"rawOutput":{"subagent_id":"sub-failed"}}`),
			},
		}
		events[5] = events[5].WithTool("compozy__subagent_delegate", nil, false)
		events[6] = events[6].WithTool("compozy__subagent_delegate", nil, true)
		var stored []store.SessionEvent
		for i, event := range events {
			event.SessionID, event.TurnID = "parent", "turn"
			content, err := MarshalAgentEvent(event)
			if err != nil {
				t.Fatal(err)
			}
			stored = append(
				stored,
				store.SessionEvent{
					ID:        fmt.Sprintf("event-%d", i),
					SessionID: "parent",
					TurnID:    "turn",
					Sequence:  int64(i + 1),
					Content:   content,
					Type:      event.Type,
				},
			)
		}
		messages, err := ToUIMessages(stored)
		if err != nil {
			t.Fatal(err)
		}
		var texts, parents, cards, positions []string
		foundInnerTool, foundFailure := false, false
		for _, message := range messages {
			for _, part := range message.Parts {
				if part.Type == "text" {
					texts = append(texts, part.Text)
					positions = append(positions, part.Text)
					parents = append(parents, part.ParentToolCallID)
				}
				if part.ToolCallID == "inner-read" {
					foundInnerTool = part.ParentToolCallID == "native"
				}
				if part.ToolCallID == "failed" {
					foundFailure = part.State == "output-error"
				}
				if part.Type == "data-compozy-subagent" {
					var payload UISubagentPayload
					if err := json.Unmarshal(part.Data, &payload); err != nil {
						t.Fatal(err)
					}
					if payload.SubagentID != part.ID || payload.ToolCallID == "" || payload.TurnID != "turn" {
						t.Fatalf("invalid card: %#v", part)
					}
					if payload.Origin == "provider_native" && payload.SubagentID != "sub-b5de770448cb8f99" {
						t.Fatalf("native id = %s", payload.SubagentID)
					}
					cards = append(cards, payload.Origin)
					positions = append(positions, payload.Origin)
				}
			}
		}
		if !slices.Equal(texts, []string{"Before", "Inner", "After"}) ||
			!slices.Equal(parents, []string{"", "native", ""}) {
			t.Fatalf("texts/parents = %v / %v", texts, parents)
		}
		if !slices.Equal(cards, []string{"provider_native", "delegated"}) || !foundInnerTool || !foundFailure {
			t.Fatalf("cards=%v inner=%v failed=%v", cards, foundInnerTool, foundFailure)
		}
		if !slices.Equal(positions, []string{"Before", "provider_native", "Inner", "After", "delegated"}) {
			t.Fatalf("part positions = %v", positions)
		}
	})
	// UT-038: card data carries the originating parent turn independently of message IDs.
	for _, tc := range []struct {
		name  string
		event acp.AgentEvent
	}{
		{name: "Agent call", event: acp.AgentEvent{Type: acp.EventTypeToolCall}.WithProviderToolMetadata("", "Agent", "pending")},
		{name: "Task call", event: acp.AgentEvent{Type: acp.EventTypeToolCall}.WithProviderToolMetadata("", "Task", "pending")},
		{name: "delegated result", event: acp.AgentEvent{Type: acp.EventTypeToolResult,
			Raw: json.RawMessage(`{"rawOutput":{"subagent_id":"sub-delegated","turn_id":"untrusted-output-turn"}}`),
		}.WithTool("compozy__subagent_delegate", nil, false)},
	} {
		t.Run("Should preserve the parent turn for "+tc.name, func(t *testing.T) {
			t.Parallel()
			tc.event.SessionID, tc.event.TurnID, tc.event.ToolCallID = "parent", "parent-turn", "call"
			content, err := MarshalAgentEvent(tc.event)
			if err != nil {
				t.Fatal(err)
			}
			messages, err := ToUIMessages([]store.SessionEvent{{
				ID: "unrelated-event-id", SessionID: "parent", TurnID: "parent-turn",
				Sequence: 1, Content: content, Type: tc.event.Type,
			}})
			if err != nil {
				t.Fatal(err)
			}
			cards := 0
			for _, message := range messages {
				for _, part := range message.Parts {
					if part.Type != "data-compozy-subagent" {
						continue
					}
					cards++
					var data map[string]any
					if err := json.Unmarshal(part.Data, &data); err != nil {
						t.Fatal(err)
					}
					if data["turn_id"] != "parent-turn" {
						t.Fatalf("card data = %s", part.Data)
					}
				}
			}
			if cards != 1 {
				t.Fatalf("cards = %d, want 1", cards)
			}
		})
	}
}

// Invariant: one authored send projects one updatable card with typed reply metadata; owner: transcript.
func TestSessionMessageUIProjection(t *testing.T) {
	t.Run("Should keep a send made inside a provider-native subagent attributed to it", func(t *testing.T) {
		t.Parallel()
		call := acp.AgentEvent{Type: acp.EventTypeToolCall, ToolCallID: "child-send", TurnID: "turn"}.WithTool(
			"compozy__session_prompt",
			json.RawMessage(`{"session_id":"target","message_id":"message"}`),
			false,
		).WithProviderToolMetadata("native-task", "compozy__session_prompt", "")
		content, err := MarshalAgentEvent(call)
		if err != nil {
			t.Fatal(err)
		}
		messages, err := ToUIMessages([]store.SessionEvent{{
			ID: "event-0", SessionID: "sender", TurnID: "turn", Sequence: 1, Type: call.Type, Content: content,
		}})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, message := range messages {
			for _, part := range message.Parts {
				if part.Type != "data-compozy-session-message" {
					continue
				}
				found = true
				if part.ParentToolCallID != "native-task" {
					t.Fatalf("session message part = %+v, want parent attribution", part)
				}
			}
		}
		if !found {
			t.Fatal("no session message part")
		}
	})

	t.Run("Should replace the running card with its correlated result UT-014", func(t *testing.T) {
		t.Parallel()
		call := acp.AgentEvent{
			Type:       acp.EventTypeToolCall,
			ToolCallID: "send",
			TurnID:     "turn",
		}.WithTool(
			"compozy__session_prompt",
			json.RawMessage(`{"session_id":"target","message_id":"message","mode":"steer","in_reply_to":"msg-plan"}`),
			false,
		)
		result := acp.AgentEvent{
			Type:       acp.EventTypeToolResult,
			ToolCallID: "send",
			TurnID:     "turn",
			Raw: json.RawMessage(
				`{"rawOutput":{"prompt":{"message_id":"message","mode":"steer","delivery":"interrupt_then_prompt","steer_delivery":"interrupt_fallback","reply_watch":{"id":"rw-123","state":"armed"},"target_workspace_id":"ws-target"}}}`,
			),
		}.WithTool(
			"compozy__session_prompt",
			nil,
			false,
		)
		var rows []store.SessionEvent
		for i, event := range []acp.AgentEvent{call, result} {
			content, err := MarshalAgentEvent(event)
			if err != nil {
				t.Fatal(err)
			}
			rows = append(
				rows,
				store.SessionEvent{
					ID:        fmt.Sprintf("event-%d", i),
					SessionID: "sender",
					TurnID:    "turn",
					Sequence:  int64(i + 1),
					Type:      event.Type,
					Content:   content,
				},
			)
		}
		messages, err := ToUIMessages(rows)
		if err != nil {
			t.Fatal(err)
		}
		cards := 0
		for _, message := range messages {
			for _, part := range message.Parts {
				if part.Type != "data-compozy-session-message" {
					continue
				}
				cards++
				var p UISessionMessagePayload
				if err := json.Unmarshal(part.Data, &p); err != nil {
					t.Fatal(err)
				}
				if p.TargetSessionID != "target" || p.MessageID != "message" || p.Mode != "steer" ||
					p.Delivery != "interrupt_then_prompt" || p.SteerDelivery != "interrupt_fallback" ||
					p.ReplyWatchID != "rw-123" ||
					p.InReplyTo != "msg-plan" ||
					p.TargetWorkspaceID != "ws-target" ||
					p.State != "done" ||
					part.ID != "send" {
					t.Fatalf("card = %+v", p)
				}
			}
		}
		if cards != 1 {
			t.Fatalf("cards = %d", cards)
		}
	})
	t.Run("Should expose failure state without inventing a reply watch UT-014", func(t *testing.T) {
		t.Parallel()
		call := acp.AgentEvent{Type: acp.EventTypeToolCall, ToolCallID: "send", TurnID: "turn"}.WithTool(
			"mcp__compozy-hosted-tools__compozy__session_prompt",
			json.RawMessage(`{"session_id":"target","message_id":"message"}`),
			false,
		)
		failure := acp.AgentEvent{
			Type:       acp.EventTypeToolResult,
			ToolCallID: "send",
			TurnID:     "turn",
		}.WithTool(
			"mcp__compozy-hosted-tools__compozy__session_prompt",
			nil,
			true,
		)
		var cards SessionMessageCards
		var p UISessionMessagePayload
		var ok bool
		for i, event := range []acp.AgentEvent{call, failure} {
			content, err := MarshalAgentEvent(event)
			if err != nil {
				t.Fatal(err)
			}
			p, ok = cards.Card(
				store.SessionEvent{Type: event.Type, TurnID: "turn", Sequence: int64(i + 1), Content: content},
			)
		}
		if !ok || p.State != "error" || p.ReplyWatchID != "" || p.TargetSessionID != "target" {
			t.Fatalf("failure card = %+v, %v", p, ok)
		}
	})
	t.Run("Should preserve typed synthetic reply metadata UT-013", func(t *testing.T) {
		t.Parallel()
		event := acp.AgentEvent{
			Type:   acp.EventTypeSyntheticReentry,
			TurnID: "reply-turn",
			Text:   "reply",
			Synthetic: &acp.PromptSyntheticMeta{
				Kind:             acp.PromptSyntheticKindSessionReply,
				WakeEventID:      "rw-123",
				ChildSessionID:   "target",
				ChildWorkspaceID: "workspace",
				ChildAgentName:   "coder",
				Reason:           "completed",
				Hop:              2,
				ReplyTruncated:   true,
			},
		}
		content, err := MarshalAgentEvent(event)
		if err != nil {
			t.Fatal(err)
		}
		messages, err := ToUIMessages(
			[]store.SessionEvent{
				{
					ID:        "reply",
					SessionID: "sender",
					TurnID:    "reply-turn",
					Sequence:  1,
					Type:      event.Type,
					Content:   content,
				},
			},
		)
		if err != nil || len(messages) != 1 {
			t.Fatalf("messages = %+v, %v", messages, err)
		}
		var meta struct {
			Synthetic *acp.PromptSyntheticMeta `json:"synthetic"`
		}
		if err := json.Unmarshal(messages[0].Metadata, &meta); err != nil {
			t.Fatal(err)
		}
		if messages[0].Role != UIRoleSystem || meta.Synthetic == nil || meta.Synthetic.Hop != 2 ||
			meta.Synthetic.WakeEventID != "rw-123" ||
			meta.Synthetic.ChildWorkspaceID != "workspace" ||
			!meta.Synthetic.ReplyTruncated {
			t.Fatalf("metadata = %+v", meta.Synthetic)
		}
	})

	t.Run("Should project a monitor wake summary and keep it off other synthetic kinds", func(t *testing.T) {
		t.Parallel()
		summaryOf := func(kind string) string {
			t.Helper()
			event := acp.AgentEvent{
				Type: acp.EventTypeSyntheticReentry, TurnID: "wake-turn", Text: "wake",
				Synthetic: &acp.PromptSyntheticMeta{
					Kind: kind, Reason: "monitor_exit", WakeEventID: "mon-0123456789ab", Summary: "release workflow",
				},
			}
			content, err := MarshalAgentEvent(event)
			if err != nil {
				t.Fatal(err)
			}
			messages, err := ToUIMessages([]store.SessionEvent{{
				ID: "wake", SessionID: "sess", TurnID: "wake-turn", Sequence: 1, Type: event.Type, Content: content,
			}})
			if err != nil || len(messages) != 1 {
				t.Fatalf("messages = %+v, %v", messages, err)
			}
			var meta struct {
				Synthetic *acp.PromptSyntheticMeta `json:"synthetic"`
			}
			if err := json.Unmarshal(messages[0].Metadata, &meta); err != nil || meta.Synthetic == nil {
				t.Fatalf("metadata = %s, %v", messages[0].Metadata, err)
			}
			return meta.Synthetic.Summary
		}
		if got := summaryOf(acp.PromptSyntheticKindMonitor); got != "release workflow" {
			t.Fatalf("monitor summary = %q, want description", got)
		}
		if got := summaryOf("task_wake"); got != "" {
			t.Fatalf("task_wake summary = %q, want omitted", got)
		}
	})

	// Recorded provider shapes (#742), captured from real Claude and Codex sessions and sanitized:
	// the card must not depend on one provider's wire form.
	t.Run("Should name a Claude send once its streamed arguments do and read its string result", func(t *testing.T) {
		t.Parallel()
		rows := recordedSessionPromptRows(t, "claude.jsonl")
		if cards := sessionMessageCardsOf(t, rows[:1]); len(cards) != 0 {
			t.Fatalf("argument-less call projected cards %+v; want the tool row alone", cards)
		}
		assertSessionMessageCards(t, rows, UISessionMessagePayload{
			ToolCallID: "toolu_01L3DE1yXY8CcKF8hkd3k8Z5", TargetSessionID: "sess-844b851f052bd406",
			TargetWorkspaceID: "ws_a5a306022a1c98a1", MessageID: "issue742-goal-unavailable-v1", Mode: "queue",
			Delivery: "direct", ReplyWatchID: "rw-f9a45066bce98c57", State: "done",
		})
	})

	t.Run("Should project Codex sends reported as an MCP call envelope with a nameless result", func(t *testing.T) {
		t.Parallel()
		assertSessionMessageCards(t, recordedSessionPromptRows(t, "codex.jsonl"),
			UISessionMessagePayload{
				ToolCallID: "exec-2536d1eb-7fab-4bd7-b971-e62f6b970e22", TargetSessionID: "sess-844b851f052bd406",
				TargetWorkspaceID: "ws_a5a306022a1c98a1", MessageID: "736-review-remediation-1", Mode: "queue",
				Delivery: "after_turn", State: "done",
			},
			UISessionMessagePayload{
				ToolCallID: "exec-5f2789fa-43ce-4158-a1b6-190745d05f03", TargetSessionID: "sess-844b851f052bd406",
				TargetWorkspaceID: "ws_a5a306022a1c98a1", MessageID: "736-fixture-remediation-1518", Mode: "steer",
				Delivery: "interrupt_then_prompt", SteerDelivery: "injected", State: "done",
			},
		)
	})
}

// recordedSessionPromptRows loads a recorded provider stream from
// testdata/session-prompt/<name>: one {"type", "content"} event per line, in sequence order.
func recordedSessionPromptRows(t *testing.T, name string) []store.SessionEvent {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "session-prompt", name))
	if err != nil {
		t.Fatal(err)
	}
	var rows []store.SessionEvent
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		var recorded struct {
			Type    string          `json:"type"`
			Content json.RawMessage `json:"content"`
		}
		if err := json.Unmarshal([]byte(line), &recorded); err != nil {
			t.Fatal(err)
		}
		var turn struct {
			TurnID string `json:"turn_id"`
		}
		if err := json.Unmarshal(recorded.Content, &turn); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, store.SessionEvent{
			ID:        fmt.Sprintf("event-%d", len(rows)),
			SessionID: "sender",
			TurnID:    turn.TurnID,
			Sequence:  int64(len(rows) + 1),
			Type:      recorded.Type,
			Content:   string(recorded.Content),
		})
	}
	return rows
}

func sessionMessageCardsOf(t *testing.T, rows []store.SessionEvent) []UISessionMessagePayload {
	t.Helper()
	messages, err := ToUIMessages(rows)
	if err != nil {
		t.Fatal(err)
	}
	var cards []UISessionMessagePayload
	for _, message := range messages {
		for _, part := range message.Parts {
			if part.Type != "data-compozy-session-message" {
				continue
			}
			var card UISessionMessagePayload
			if err := json.Unmarshal(part.Data, &card); err != nil {
				t.Fatal(err)
			}
			cards = append(cards, card)
		}
	}
	return cards
}

// assertSessionMessageCards checks the transcript's cards and the live stream's (the prompt
// delivery folds the same events through SessionMessageCards) against want, in call order.
func assertSessionMessageCards(t *testing.T, rows []store.SessionEvent, want ...UISessionMessagePayload) {
	t.Helper()
	if cards := sessionMessageCardsOf(t, rows); !slices.Equal(cards, want) {
		t.Fatalf("transcript cards = %+v, want %+v", cards, want)
	}
	var live SessionMessageCards
	latest := make(map[string]UISessionMessagePayload)
	var order []string
	for _, row := range rows {
		card, ok := live.Card(row)
		if !ok {
			continue
		}
		if _, seen := latest[card.ToolCallID]; !seen {
			order = append(order, card.ToolCallID)
		}
		latest[card.ToolCallID] = card
	}
	got := make([]UISessionMessagePayload, 0, len(order))
	for _, id := range order {
		got = append(got, latest[id])
	}
	if !slices.Equal(got, want) {
		t.Fatalf("live cards = %+v, want %+v", got, want)
	}
}

// UT-013: operator inputs remain unattributed while session origins reach the UI.
func TestPromptOriginUIProjection(t *testing.T) {
	t.Parallel()
	t.Run("Should project daemon origin while leaving operator metadata unattributed", func(t *testing.T) {
		t.Parallel()
		origin := &acp.PromptOriginMeta{Kind: "session", SessionID: "sess-a", WorkspaceID: "ws-a", Hop: 2}
		event := (acp.AgentEvent{Type: acp.EventTypeUserMessage, TurnID: "turn-a", Text: "question"}).WithPromptOrigin(
			origin,
		)
		var got struct {
			Origin *acp.PromptOriginMeta `json:"origin"`
		}
		if err := json.Unmarshal(inputUIMessageMetadata(event), &got); err != nil {
			t.Fatal(err)
		}
		if got.Origin == nil || *got.Origin != *origin {
			t.Fatal(got)
		}
		if strings.Contains(string(inputUIMessageMetadata(event.WithPromptOrigin(nil))), `"origin"`) {
			t.Fatal("operator acquired origin")
		}
	})
}

// UT-013: the transcript exposes only typed UI metadata; reply bodies remain available.
func TestSyntheticUIMetadataBoundary(t *testing.T) {
	for _, kind := range []string{"session_reply", "task_wake", "subagent_wake"} {
		t.Run("Should project only UI fields for "+kind, func(t *testing.T) {
			t.Parallel()
			event := acp.AgentEvent{Synthetic: &acp.PromptSyntheticMeta{
				Kind:                 kind,
				WakeEventID:          "wake",
				ChildSessionID:       "child",
				ChildWorkspaceID:     "ws",
				ChildAgentName:       "coder",
				Reason:               "completed",
				ReplyTruncated:       true,
				Hop:                  2,
				Summary:              "answer",
				TaskID:               "task",
				TaskRunID:            "run",
				ClaimTokenHash:       "private-hash",
				SubagentIDs:          []string{"subagent"},
				WorkflowID:           "workflow",
				Badge:                "badge",
				CoordinatorSessionID: "coordinator",
				PolicySnapshotID:     "policy",
				PolicyDigest:         "digest",
				ConfigDigest:         "config",
			}}
			var meta struct {
				Synthetic map[string]json.RawMessage `json:"synthetic"`
			}
			if err := json.Unmarshal(inputUIMessageMetadata(event), &meta); err != nil {
				t.Fatal(err)
			}
			expected := []string{
				"kind",
				"wake_event_id",
				"child_session_id",
				"child_workspace_id",
				"child_agent_name",
				"reason",
				"reply_truncated",
				"hop",
			}
			if kind == "session_reply" {
				expected = append(expected, "summary")
			}
			if len(meta.Synthetic) != len(expected) {
				t.Fatalf("metadata = %s", inputUIMessageMetadata(event))
			}
			for _, key := range expected {
				if _, ok := meta.Synthetic[key]; !ok {
					t.Errorf("missing %s", key)
				}
			}
			if kind == "session_reply" && string(meta.Synthetic["summary"]) != `"answer"` {
				t.Fatal("reply body missing")
			}
		})
	}
}
