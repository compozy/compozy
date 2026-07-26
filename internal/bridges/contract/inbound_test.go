package contract

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestInboundInteractionContracts(t *testing.T) {
	t.Parallel()

	t.Run("Should expose and validate every closed interaction enum", func(t *testing.T) {
		t.Parallel()

		if got, want := strings.Join(
			InboundEventFamilyValues(),
			",",
		), "message,command,action,reaction,edit"; got != want {
			t.Fatalf("InboundEventFamilyValues() = %q, want %q", got, want)
		}
		families := []struct {
			name  string
			value InboundEventFamily
		}{
			{name: "Should accept message events", value: InboundEventFamilyMessage},
			{name: "Should accept command events", value: InboundEventFamilyCommand},
			{name: "Should accept action events", value: InboundEventFamilyAction},
			{name: "Should accept reaction events", value: InboundEventFamilyReaction},
			{name: "Should accept edit events", value: InboundEventFamilyEdit},
		}
		for _, test := range families {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				if err := test.value.Validate(); err != nil {
					t.Fatalf("InboundEventFamily(%q).Validate() error = %v", test.value, err)
				}
			})
		}
		if err := (InboundEventFamily("unknown")).Validate(); err == nil {
			t.Fatal("InboundEventFamily(unknown).Validate() error = nil")
		}
		if err := (InboundEventFamily("")).Validate(); err == nil {
			t.Fatal("InboundEventFamily(empty).Validate() error = nil")
		}

		if got, want := strings.Join(InboundEditOperationValues(), ","), "updated,deleted"; got != want {
			t.Fatalf("InboundEditOperationValues() = %q, want %q", got, want)
		}
		operations := []struct {
			name  string
			value InboundEditOperation
		}{
			{name: "Should accept updated edits", value: InboundEditOperationUpdated},
			{name: "Should accept deleted edits", value: InboundEditOperationDeleted},
		}
		for _, test := range operations {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				if err := test.value.Validate(); err != nil {
					t.Fatalf("InboundEditOperation(%q).Validate() error = %v", test.value, err)
				}
			})
		}
		if err := (InboundEditOperation("unknown")).Validate(); err == nil {
			t.Fatal("InboundEditOperation(unknown).Validate() error = nil")
		}
	})

	t.Run("Should validate typed payload identities and edit semantics", func(t *testing.T) {
		t.Parallel()

		if err := (InboundCommand{Command: " /deploy "}).Validate(); err != nil {
			t.Fatalf("InboundCommand.Validate() error = %v", err)
		}
		if err := (InboundCommand{}).Validate(); err == nil {
			t.Fatal("InboundCommand.Validate(empty) error = nil")
		}
		if err := (InboundAction{ActionID: " approve "}).Validate(); err != nil {
			t.Fatalf("InboundAction.Validate() error = %v", err)
		}
		if err := (InboundAction{}).Validate(); err == nil {
			t.Fatal("InboundAction.Validate(empty) error = nil")
		}
		if err := (InboundReaction{MessageID: "msg-1", Emoji: "thumbsup"}).Validate(); err != nil {
			t.Fatalf("InboundReaction.Validate() error = %v", err)
		}
		if err := (InboundReaction{Emoji: "thumbsup"}).Validate(); err == nil {
			t.Fatal("InboundReaction.Validate(missing message) error = nil")
		}
		if err := (InboundReaction{MessageID: "msg-1"}).Validate(); err == nil {
			t.Fatal("InboundReaction.Validate(missing emoji) error = nil")
		}

		timestamp := time.Date(2026, 7, 13, 12, 0, 0, 0, time.FixedZone("offset", 3600))
		updated := InboundEdit{
			MessageID: " msg-1 ", NewText: " corrected ", OriginalTimestamp: timestamp,
			Operation: " UPDATED ",
		}
		if err := updated.Validate(); err != nil {
			t.Fatalf("InboundEdit(updated).Validate() error = %v", err)
		}
		if normalized := updated.normalize(); normalized.MessageID != "msg-1" || normalized.NewText != "corrected" ||
			normalized.Operation != InboundEditOperationUpdated || normalized.OriginalTimestamp.Location() != time.UTC {
			t.Fatalf("InboundEdit.normalize() = %#v, want canonical update", normalized)
		}
		if err := (InboundEdit{MessageID: "msg-1", OriginalTimestamp: timestamp, Operation: InboundEditOperationDeleted}).Validate(); err != nil {
			t.Fatalf("InboundEdit(deleted).Validate() error = %v", err)
		}
		invalid := []struct {
			name string
			edit InboundEdit
		}{
			{
				name: "Should reject an edit without a message id",
				edit: InboundEdit{OriginalTimestamp: timestamp, Operation: InboundEditOperationDeleted},
			},
			{
				name: "Should reject an edit without a timestamp",
				edit: InboundEdit{MessageID: "msg-1", Operation: InboundEditOperationDeleted},
			},
			{
				name: "Should reject an edit without an operation",
				edit: InboundEdit{MessageID: "msg-1", OriginalTimestamp: timestamp},
			},
			{
				name: "Should reject an updated edit without text",
				edit: InboundEdit{
					MessageID:         "msg-1",
					OriginalTimestamp: timestamp,
					Operation:         InboundEditOperationUpdated,
				},
			},
			{
				name: "Should reject a deleted edit with text",
				edit: InboundEdit{
					MessageID:         "msg-1",
					NewText:           "not allowed",
					OriginalTimestamp: timestamp,
					Operation:         InboundEditOperationDeleted,
				},
			},
		}
		for _, test := range invalid {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				if err := test.edit.Validate(); err == nil {
					t.Fatalf("InboundEdit(%#v).Validate() error = nil", test.edit)
				}
			})
		}
	})
}

func TestInboundMessageEnvelopeContract(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize a message with independently owned mutable payloads", func(t *testing.T) {
		t.Parallel()

		message := richInboundMessageEnvelope()
		if err := message.Validate(); err != nil {
			t.Fatalf("message.Validate() error = %v", err)
		}
		ref, ok, err := message.NetworkConversationRef()
		if err != nil || !ok || ref.Channel != "agh" || ref.ThreadID != "thread_123" {
			t.Fatalf("NetworkConversationRef() = (%#v, %v, %v), want canonical thread ref", ref, ok, err)
		}
		normalized := NormalizeInboundMessageEnvelope(message)
		normalized.Attachments[0].Name = "mutated"
		normalized.ProviderMetadata[0] = '['
		normalized.Conversation.ThreadID = "thread_mutated"
		if message.Attachments[0].Name != " report.pdf " {
			t.Fatalf("NormalizeInboundMessageEnvelope() aliased source attachments: %#v", message.Attachments)
		}
		if got, want := string(message.ProviderMetadata), `{"provider":"slack"}`; got != want {
			t.Fatalf("source.ProviderMetadata = %q after normalized mutation, want %q", got, want)
		}
		if got, want := message.Conversation.ThreadID, " thread_123 "; got != want {
			t.Fatalf("source.Conversation.ThreadID = %q after normalized mutation, want %q", got, want)
		}
		empty := NormalizeInboundMessageEnvelope(InboundMessageEnvelope{})
		if empty.Attachments != nil || empty.ProviderMetadata != nil || empty.Conversation != nil {
			t.Fatalf("NormalizeInboundMessageEnvelope(zero) = %#v, want nil optional payloads", empty)
		}
	})

	t.Run("Should preserve public reply and provider metadata fields through JSON", func(t *testing.T) {
		t.Parallel()

		message := richInboundMessageEnvelope()
		payload, err := json.Marshal(message)
		if err != nil {
			t.Fatalf("json.Marshal(message) error = %v", err)
		}
		fieldNames := []struct {
			name string
			key  string
		}{
			{name: "Should encode reply text under its public name", key: `"reply_to_text"`},
			{name: "Should encode reply author id under its public name", key: `"reply_to_author_id"`},
			{name: "Should encode reply author name under its public name", key: `"reply_to_author_name"`},
		}
		for _, test := range fieldNames {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				if !strings.Contains(string(payload), test.key) {
					t.Fatalf("json.Marshal(message) = %s, want field %s", payload, test.key)
				}
			})
		}
		var decoded InboundMessageEnvelope
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatalf("json.Unmarshal(message) error = %v", err)
		}
		if err := decoded.Validate(); err != nil {
			t.Fatalf("decoded.Validate() error = %v", err)
		}
		if decoded.EventFamily != InboundEventFamilyMessage ||
			decoded.ReplyToText != message.ReplyToText ||
			decoded.ReplyToAuthorID != message.ReplyToAuthorID ||
			decoded.ReplyToAuthorName != message.ReplyToAuthorName ||
			string(decoded.ProviderMetadata) != string(message.ProviderMetadata) {
			t.Fatalf("json.Unmarshal(message) = %#v, want exact public reply and metadata fields", decoded)
		}
	})

	t.Run("Should preserve a typed family and raw provider metadata through JSON", func(t *testing.T) {
		t.Parallel()

		envelope := typedInboundEnvelope(InboundEventFamilyAction)
		envelope.Action = &InboundAction{ActionID: "approve", MessageID: "msg-1"}
		envelope.ProviderMetadata = json.RawMessage(`{"provider":"slack","raw_action_id":"A123"}`)
		payload, err := json.Marshal(envelope)
		if err != nil {
			t.Fatalf("json.Marshal(action) error = %v", err)
		}
		var decoded InboundMessageEnvelope
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatalf("json.Unmarshal(action) error = %v", err)
		}
		if decoded.EventFamily != InboundEventFamilyAction || decoded.Action == nil ||
			decoded.Command != nil || decoded.Reaction != nil || decoded.Edit != nil ||
			string(decoded.ProviderMetadata) != string(envelope.ProviderMetadata) {
			t.Fatalf("json.Unmarshal(action) = %#v, want action family and exact provider metadata", decoded)
		}
	})

	t.Run("Should validate every typed interaction family", func(t *testing.T) {
		t.Parallel()

		command := typedInboundEnvelope(InboundEventFamilyCommand)
		command.Command = &InboundCommand{Command: "/deploy", Text: "prod", TriggerID: "trigger-1"}
		action := typedInboundEnvelope(InboundEventFamilyAction)
		action.Action = &InboundAction{ActionID: "approve", MessageID: "msg-1", Value: "yes", TriggerID: "trigger-2"}
		reaction := typedInboundEnvelope(InboundEventFamilyReaction)
		reaction.Reaction = &InboundReaction{MessageID: "msg-1", Emoji: "thumbsup", RawEmoji: ":+1:", Added: true}
		edit := typedInboundEnvelope(InboundEventFamilyEdit)
		edit.Edit = &InboundEdit{
			MessageID: "msg-1", NewText: "corrected", OriginalTimestamp: edit.ReceivedAt.Add(-time.Minute),
			Operation: InboundEditOperationUpdated,
		}
		edit.ReplyToText = "context"
		envelopes := []struct {
			name     string
			envelope InboundMessageEnvelope
		}{
			{name: "Should accept a command envelope", envelope: command},
			{name: "Should accept an action envelope", envelope: action},
			{name: "Should accept a reaction envelope", envelope: reaction},
			{name: "Should accept an edit envelope", envelope: edit},
		}
		for _, test := range envelopes {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				if err := test.envelope.Validate(); err != nil {
					t.Fatalf("InboundMessageEnvelope(%q).Validate() error = %v", test.envelope.EventFamily, err)
				}
			})
		}
	})

	t.Run("Should reject incomplete and conflicting family payloads", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			mutate func(*InboundMessageEnvelope)
		}{
			{
				name:   "Should reject a missing instance",
				mutate: func(v *InboundMessageEnvelope) { v.BridgeInstanceID = "" },
			},
			{name: "Should reject an invalid scope", mutate: func(v *InboundMessageEnvelope) { v.WorkspaceID = "" }},
			{
				name:   "Should reject a missing received timestamp",
				mutate: func(v *InboundMessageEnvelope) { v.ReceivedAt = time.Time{} },
			},
			{
				name:   "Should reject an invalid family",
				mutate: func(v *InboundMessageEnvelope) { v.EventFamily = "unknown" },
			},
			{
				name:   "Should reject invalid metadata",
				mutate: func(v *InboundMessageEnvelope) { v.ProviderMetadata = json.RawMessage(`{`) },
			},
			{
				name:   "Should reject a missing idempotency key",
				mutate: func(v *InboundMessageEnvelope) { v.IdempotencyKey = "" },
			},
			{
				name:   "Should reject a missing message id",
				mutate: func(v *InboundMessageEnvelope) { v.PlatformMessageID = "" },
			},
			{
				name:   "Should reject a message with command payload",
				mutate: func(v *InboundMessageEnvelope) { v.Command = &InboundCommand{Command: "/x"} },
			},
			{
				name:   "Should reject a command without payload",
				mutate: func(v *InboundMessageEnvelope) { *v = typedInboundEnvelope(InboundEventFamilyCommand) },
			},
			{name: "Should reject a command with message fields", mutate: func(v *InboundMessageEnvelope) {
				*v = typedInboundEnvelope(InboundEventFamilyCommand)
				v.Command = &InboundCommand{Command: "/x"}
				v.Content.Text = "forbidden"
			}},
			{name: "Should reject an action with reaction payload", mutate: func(v *InboundMessageEnvelope) {
				*v = typedInboundEnvelope(InboundEventFamilyAction)
				v.Action = &InboundAction{ActionID: "a"}
				v.Reaction = &InboundReaction{MessageID: "m", Emoji: "e"}
			}},
			{
				name:   "Should reject a reaction without payload",
				mutate: func(v *InboundMessageEnvelope) { *v = typedInboundEnvelope(InboundEventFamilyReaction) },
			},
			{
				name:   "Should reject an edit without payload",
				mutate: func(v *InboundMessageEnvelope) { *v = typedInboundEnvelope(InboundEventFamilyEdit) },
			},
			{name: "Should reject reply context on a command", mutate: func(v *InboundMessageEnvelope) {
				*v = typedInboundEnvelope(InboundEventFamilyCommand)
				v.Command = &InboundCommand{Command: "/x"}
				v.ReplyToText = "forbidden"
			}},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				envelope := baseInboundEnvelope()
				test.mutate(&envelope)
				if err := envelope.Validate(); err == nil {
					t.Fatalf("InboundMessageEnvelope.Validate(%s) error = nil", test.name)
				}
			})
		}
	})

	t.Run("Should validate thread and direct conversation identities", func(t *testing.T) {
		t.Parallel()

		directID := "direct_" + strings.Repeat("a", 32)
		valid := []struct {
			name string
			ref  NetworkConversationRef
		}{
			{
				name: "Should accept a thread conversation",
				ref: NetworkConversationRef{
					Channel:  "agh",
					Surface:  NetworkConversationSurfaceThread,
					ThreadID: "thread_123",
					WorkID:   "work_1",
				},
			},
			{
				name: "Should accept a direct conversation",
				ref: NetworkConversationRef{
					Channel:  "agh",
					Surface:  NetworkConversationSurfaceDirect,
					DirectID: directID,
				},
			},
		}
		for _, test := range valid {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				if err := test.ref.Validate(); err != nil {
					t.Fatalf("NetworkConversationRef(%#v).Validate() error = %v", test.ref, err)
				}
			})
		}
		invalid := []struct {
			name string
			ref  NetworkConversationRef
		}{
			{name: "Should reject an empty conversation", ref: NetworkConversationRef{}},
			{name: "Should reject an unknown surface", ref: NetworkConversationRef{Channel: "agh", Surface: "room"}},
			{
				name: "Should reject a malformed thread id",
				ref: NetworkConversationRef{
					Channel:  "agh",
					Surface:  NetworkConversationSurfaceThread,
					ThreadID: "bad",
				},
			},
			{
				name: "Should reject a direct id on a thread",
				ref: NetworkConversationRef{
					Channel:  "agh",
					Surface:  NetworkConversationSurfaceThread,
					ThreadID: "thread_1",
					DirectID: directID,
				},
			},
			{
				name: "Should reject a short direct id",
				ref: NetworkConversationRef{
					Channel:  "agh",
					Surface:  NetworkConversationSurfaceDirect,
					DirectID: "direct_short",
				},
			},
			{
				name: "Should reject a thread id on a direct conversation",
				ref: NetworkConversationRef{
					Channel:  "agh",
					Surface:  NetworkConversationSurfaceDirect,
					DirectID: directID,
					ThreadID: "thread_1",
				},
			},
			{
				name: "Should reject a malformed work id",
				ref: NetworkConversationRef{
					Channel:  "agh",
					Surface:  NetworkConversationSurfaceThread,
					ThreadID: "thread_1",
					WorkID:   "bad/id",
				},
			},
			{
				name: "Should reject control characters in a thread id",
				ref: NetworkConversationRef{
					Channel:  "agh",
					Surface:  NetworkConversationSurfaceThread,
					ThreadID: "thread_\x01",
				},
			},
		}
		for _, test := range invalid {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				if err := test.ref.Validate(); err == nil {
					t.Fatalf("NetworkConversationRef(%#v).Validate() error = nil", test.ref)
				}
			})
		}
		normalized := NormalizeNetworkConversationRef(NetworkConversationRef{
			Channel: " agh ", Surface: " THREAD ", ThreadID: " thread_123 ",
			WorkID: " work_1 ", ReplyTo: " reply-1 ", TraceID: " trace-1 ",
			CausationID: " cause-1 ",
		})
		if normalized.Channel != "agh" || normalized.Surface != NetworkConversationSurfaceThread ||
			normalized.ThreadID != "thread_123" || normalized.WorkID != "work_1" ||
			normalized.ReplyTo != "reply-1" || normalized.TraceID != "trace-1" ||
			normalized.CausationID != "cause-1" {
			t.Fatalf("NormalizeNetworkConversationRef() = %#v, want canonical conversation", normalized)
		}
		if _, ok, err := (InboundMessageEnvelope{}).NetworkConversationRef(); err != nil || ok {
			t.Fatalf("NetworkConversationRef(empty envelope) = (_, %v, %v), want false, nil", ok, err)
		}
	})

	t.Run("Should not infer an AGH conversation from provider routing fields", func(t *testing.T) {
		t.Parallel()

		envelope := baseInboundEnvelope()
		envelope.ThreadID = "provider-thread"
		ref, ok, err := envelope.NetworkConversationRef()
		if err != nil {
			t.Fatalf("NetworkConversationRef() error = %v", err)
		}
		if ok {
			t.Fatalf("NetworkConversationRef() = (%#v, true), want no inferred AGH conversation", ref)
		}
	})
}

func baseInboundEnvelope() InboundMessageEnvelope {
	return InboundMessageEnvelope{
		BridgeInstanceID: "brg-1", Scope: ScopeWorkspace, WorkspaceID: "ws-1",
		PeerID: "peer-1", ThreadID: "thread-1", GroupID: "group-1",
		PlatformMessageID: "msg-1", ReceivedAt: time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC),
		Sender:  MessageSender{ID: "user-1", Username: "alice", DisplayName: "Alice"},
		Content: MessageContent{Text: "hello"}, EventFamily: InboundEventFamilyMessage,
		IdempotencyKey: "idem-1",
	}
}

func typedInboundEnvelope(family InboundEventFamily) InboundMessageEnvelope {
	envelope := baseInboundEnvelope()
	envelope.EventFamily = family
	envelope.PlatformMessageID = ""
	envelope.Content = MessageContent{}
	envelope.Attachments = nil
	return envelope
}

func richInboundMessageEnvelope() InboundMessageEnvelope {
	envelope := baseInboundEnvelope()
	envelope.ReplyToText = " earlier "
	envelope.ReplyToAuthorID = " user-0 "
	envelope.ReplyToAuthorName = " Bob "
	envelope.Conversation = &NetworkConversationRef{
		Channel: " agh ", Surface: NetworkConversationSurfaceThread, ThreadID: " thread_123 ",
		WorkID: " work_1 ", ReplyTo: " reply-1 ", TraceID: " trace-1 ", CausationID: " cause-1 ",
	}
	envelope.ProviderMetadata = json.RawMessage(`{"provider":"slack"}`)
	envelope.Attachments = []MessageAttachment{{
		ID: " att-1 ", Name: " report.pdf ", MIMEType: " application/pdf ",
		URL: " https://example.test/report.pdf ",
	}}
	return envelope
}
