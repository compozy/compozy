// Suite: bridge delivery wire contract
// Invariant: delivery events, snapshots, requests, targets, and acknowledgements remain closed and internally consistent.
// Boundary IN: daemon-to-provider delivery payloads and provider acknowledgements.
// Boundary OUT: broker ordering, retries, persistence, and provider HTTP calls, owned by their integration suites.
package contract

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDeliveryScalarContracts(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize modes, operations, and event catalogs", func(t *testing.T) {
		t.Parallel()

		modeAliases := []struct {
			name  string
			alias DeliveryMode
			want  DeliveryMode
		}{
			{name: "Should normalize direct", alias: "direct", want: DeliveryModeDirectSend},
			{name: "Should normalize direct underscore", alias: "direct_send", want: DeliveryModeDirectSend},
			{name: "Should normalize send", alias: "send", want: DeliveryModeDirectSend},
			{name: "Should normalize reply underscore", alias: "reply_send", want: DeliveryModeReply},
		}
		for _, test := range modeAliases {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				if got := test.alias.Normalize(); got != test.want {
					t.Fatalf("DeliveryMode(%q).Normalize() = %q, want %q", test.alias, got, test.want)
				}
				if err := test.alias.Validate(); err != nil {
					t.Fatalf("DeliveryMode(%q).Validate() error = %v", test.alias, err)
				}
			})
		}

		invalidModes := []struct {
			name string
			mode DeliveryMode
		}{
			{name: "Should reject an empty delivery mode", mode: ""},
			{name: "Should reject an unsupported delivery mode", mode: "broadcast"},
		}
		for _, test := range invalidModes {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				if err := test.mode.Validate(); err == nil {
					t.Fatalf("DeliveryMode(%q).Validate() error = nil", test.mode)
				}
			})
		}

		operations := []struct {
			name      string
			operation DeliveryOperation
		}{
			{name: "Should accept an omitted post operation", operation: ""},
			{name: "Should accept a post operation", operation: DeliveryOperationPost},
			{name: "Should accept an edit operation", operation: DeliveryOperationEdit},
			{name: "Should accept a delete operation", operation: DeliveryOperationDelete},
		}
		for _, test := range operations {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				if err := test.operation.Validate(); err != nil {
					t.Fatalf("DeliveryOperation(%q).Validate() error = %v", test.operation, err)
				}
			})
		}
		t.Run("Should reject an unsupported delivery operation", func(t *testing.T) {
			t.Parallel()

			if err := (DeliveryOperation("replace")).Validate(); err == nil {
				t.Fatal("DeliveryOperation(replace).Validate() error = nil")
			}
		})
		if got, want := strings.Join(
			DeliveryEventTypeValues(),
			",",
		), "start,delta,final,error,resume,delete,progress"; got != want {
			t.Fatalf("DeliveryEventTypeValues() = %q, want %q", got, want)
		}
	})

	t.Run("Should validate delivery targets", func(t *testing.T) {
		t.Parallel()

		target := DeliveryTarget{
			BridgeInstanceID: " brg-1 ",
			PeerID:           " peer-1 ",
			ThreadID:         " thread-1 ",
			Mode:             " REPLY ",
		}
		if err := target.Validate(); err != nil {
			t.Fatalf("DeliveryTarget.Validate() error = %v", err)
		}
		if target.IsZero() || !(DeliveryTarget{}).IsZero() {
			t.Fatalf(
				"DeliveryTarget.IsZero() values = %v/%v, want false/true",
				target.IsZero(),
				(DeliveryTarget{}).IsZero(),
			)
		}
		normalized := NormalizeDeliveryTarget(target)
		if normalized.BridgeInstanceID != "brg-1" || normalized.PeerID != "peer-1" ||
			normalized.ThreadID != "thread-1" || normalized.Mode != DeliveryModeReply {
			t.Fatalf("NormalizeDeliveryTarget() = %#v, want canonical target", normalized)
		}
		invalidTargets := []struct {
			name   string
			target DeliveryTarget
		}{
			{
				name:   "Should require a target bridge instance",
				target: DeliveryTarget{PeerID: "peer-1", Mode: DeliveryModeReply},
			},
			{
				name:   "Should require a target delivery mode",
				target: DeliveryTarget{BridgeInstanceID: "brg-1", PeerID: "peer-1"},
			},
			{
				name:   "Should reject a thread without an anchor",
				target: DeliveryTarget{BridgeInstanceID: "brg-1", ThreadID: "thread-1", Mode: DeliveryModeReply},
			},
			{
				name:   "Should require a direct-send destination",
				target: DeliveryTarget{BridgeInstanceID: "brg-1", Mode: DeliveryModeDirectSend},
			},
		}
		for _, test := range invalidTargets {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				if err := test.target.Validate(); err == nil {
					t.Fatalf("DeliveryTarget(%#v).Validate() error = nil", test.target)
				}
			})
		}
	})

	t.Run("Should validate message references and error details", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			validate func() error
			wantErr  bool
		}{
			{
				name:     "Should accept a delivery-id reference",
				validate: func() error { return (DeliveryMessageReference{DeliveryID: " del-1 "}).Validate() },
			},
			{
				name:     "Should accept a remote-message reference",
				validate: func() error { return (DeliveryMessageReference{RemoteMessageID: " remote-1 "}).Validate() },
			},
			{
				name:     "Should reject an empty message reference",
				validate: func() error { return (DeliveryMessageReference{}).Validate() },
				wantErr:  true,
			},
			{
				name:     "Should accept a populated error detail",
				validate: func() error { return (DeliveryErrorDetail{Message: " failed "}).Validate() },
			},
			{
				name:     "Should reject an empty error detail",
				validate: func() error { return (DeliveryErrorDetail{}).Validate() },
				wantErr:  true,
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				err := test.validate()
				if test.wantErr && err == nil {
					t.Fatalf("validation error = nil for %s", test.name)
				}
				if !test.wantErr && err != nil {
					t.Fatalf("validation error = %v for %s", err, test.name)
				}
			})
		}
		normalized := NormalizeDeliveryMessageReference(DeliveryMessageReference{
			DeliveryID: " del-1 ", RemoteMessageID: " remote-1 ",
		})
		if normalized.DeliveryID != "del-1" || normalized.RemoteMessageID != "remote-1" {
			t.Fatalf("NormalizeDeliveryMessageReference() = %#v, want canonical reference", normalized)
		}
	})

	t.Run("Should normalize and classify delivery event types", func(t *testing.T) {
		t.Parallel()

		if got, want := NormalizeDeliveryEventType(" FINAL "), DeliveryEventTypeFinal; got != want {
			t.Fatalf("NormalizeDeliveryEventType() = %q, want %q", got, want)
		}
		cases := []struct {
			name      string
			eventType DeliveryEventType
			terminal  bool
		}{
			{name: "Should classify final as terminal", eventType: DeliveryEventTypeFinal, terminal: true},
			{name: "Should classify error as terminal", eventType: " ERROR ", terminal: true},
			{name: "Should classify delete as terminal", eventType: DeliveryEventTypeDelete, terminal: true},
			{name: "Should classify start as non-terminal", eventType: DeliveryEventTypeStart},
			{name: "Should classify progress as non-terminal", eventType: DeliveryEventTypeProgress},
		}
		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				if got := IsTerminalDeliveryEventType(test.eventType); got != test.terminal {
					t.Fatalf("IsTerminalDeliveryEventType(%q) = %t, want %t", test.eventType, got, test.terminal)
				}
			})
		}
	})

	t.Run("Should validate resume predecessor state", func(t *testing.T) {
		t.Parallel()

		validResumeTypes := []DeliveryEventType{
			DeliveryEventTypeStart, DeliveryEventTypeDelta, DeliveryEventTypeFinal,
			DeliveryEventTypeError, DeliveryEventTypeDelete, DeliveryEventTypeProgress,
		}
		for _, latest := range validResumeTypes {
			t.Run("Should accept resume state after "+string(latest), func(t *testing.T) {
				t.Parallel()

				if err := (DeliveryResumeState{LatestEventType: latest}).Validate(); err != nil {
					t.Fatalf("DeliveryResumeState(%q).Validate() error = %v", latest, err)
				}
			})
		}
		invalidResumeTypes := []struct {
			name   string
			latest DeliveryEventType
		}{
			{name: "Should require a resume predecessor", latest: ""},
			{name: "Should reject resume as its own predecessor", latest: DeliveryEventTypeResume},
			{name: "Should reject an unsupported resume predecessor", latest: "unknown"},
		}
		for _, test := range invalidResumeTypes {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				if err := (DeliveryResumeState{LatestEventType: test.latest}).Validate(); err == nil {
					t.Fatalf("DeliveryResumeState(%q).Validate() error = nil", test.latest)
				}
			})
		}
	})

	t.Run("Should validate delivery acknowledgements", func(t *testing.T) {
		t.Parallel()

		event := validDeliveryEvent(DeliveryEventTypeDelta)
		tests := []struct {
			name    string
			ack     DeliveryAck
			wantErr bool
		}{
			{name: "Should reject an empty acknowledgement", ack: DeliveryAck{}, wantErr: true},
			{
				name:    "Should require a delivery id",
				ack:     DeliveryAck{Seq: event.Seq},
				wantErr: true,
			},
			{
				name: "Should accept a matching acknowledgement",
				ack:  DeliveryAck{DeliveryID: " del-1 ", Seq: 2, RemoteMessageID: " remote-1 "},
			},
			{
				name: "Should accept a committed result without an addressable remote identity",
				ack: DeliveryAck{
					DeliveryID: "del-1",
					Seq:        2,
					Outcome:    DeliveryAckOutcomeCommittedResultUnavailable,
					Error:      &DeliveryErrorDetail{Message: " response omitted id "},
				},
			},
			{
				name:    "Should reject a mismatched delivery id",
				ack:     DeliveryAck{DeliveryID: "other", Seq: event.Seq},
				wantErr: true,
			},
			{
				name:    "Should reject an omitted sequence for a nonzero event",
				ack:     DeliveryAck{DeliveryID: event.DeliveryID},
				wantErr: true,
			},
			{
				name:    "Should reject a mismatched sequence",
				ack:     DeliveryAck{DeliveryID: event.DeliveryID, Seq: 99},
				wantErr: true,
			},
			{
				name:    "Should reject an unsupported outcome",
				ack:     DeliveryAck{DeliveryID: event.DeliveryID, Seq: event.Seq, Outcome: "unknown"},
				wantErr: true,
			},
			{
				name: "Should require an error for an unavailable committed result",
				ack: DeliveryAck{
					DeliveryID: event.DeliveryID,
					Seq:        event.Seq,
					Outcome:    DeliveryAckOutcomeCommittedResultUnavailable,
				},
				wantErr: true,
			},
			{
				name: "Should reject an empty error for an unavailable committed result",
				ack: DeliveryAck{
					DeliveryID: event.DeliveryID,
					Seq:        event.Seq,
					Outcome:    DeliveryAckOutcomeCommittedResultUnavailable,
					Error:      &DeliveryErrorDetail{},
				},
				wantErr: true,
			},
			{
				name: "Should reject a remote id for an unavailable committed result",
				ack: DeliveryAck{
					DeliveryID:      event.DeliveryID,
					Seq:             event.Seq,
					Outcome:         DeliveryAckOutcomeCommittedResultUnavailable,
					Error:           &DeliveryErrorDetail{Message: "response omitted id"},
					RemoteMessageID: "fabricated",
				},
				wantErr: true,
			},
			{
				name: "Should reject a replacement id for an unavailable committed result",
				ack: DeliveryAck{
					DeliveryID:             event.DeliveryID,
					Seq:                    event.Seq,
					Outcome:                DeliveryAckOutcomeCommittedResultUnavailable,
					Error:                  &DeliveryErrorDetail{Message: "response omitted id"},
					ReplaceRemoteMessageID: "fabricated",
				},
				wantErr: true,
			},
			{
				name: "Should reject an error on a successful acknowledgement",
				ack: DeliveryAck{
					DeliveryID: event.DeliveryID,
					Seq:        event.Seq,
					Error:      &DeliveryErrorDetail{Message: "contradiction"},
				},
				wantErr: true,
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				err := test.ack.ValidateFor(event)
				if test.wantErr && err == nil {
					t.Fatalf("DeliveryAck.ValidateFor(%s) error = nil", test.name)
				}
				if !test.wantErr && err != nil {
					t.Fatalf("DeliveryAck.ValidateFor(%s) error = %v", test.name, err)
				}
			})
		}
		t.Run("Should accept an exact zero sequence", func(t *testing.T) {
			t.Parallel()

			zeroSeqEvent := event
			zeroSeqEvent.Seq = 0
			ack := DeliveryAck{DeliveryID: zeroSeqEvent.DeliveryID, Seq: 0}
			if err := ack.ValidateFor(zeroSeqEvent); err != nil {
				t.Fatalf("DeliveryAck.ValidateFor() error = %v, want nil", err)
			}
		})
		t.Run("Should reject a nonzero sequence for a zero-sequence event", func(t *testing.T) {
			t.Parallel()

			zeroSeqEvent := event
			zeroSeqEvent.Seq = 0
			ack := DeliveryAck{DeliveryID: zeroSeqEvent.DeliveryID, Seq: 1}
			if err := ack.ValidateFor(zeroSeqEvent); err == nil {
				t.Fatal("DeliveryAck.ValidateFor() error = nil, want sequence mismatch")
			}
		})
		t.Run("Should serialize required identity for a zero-sequence acknowledgement", func(t *testing.T) {
			t.Parallel()

			encoded, err := json.Marshal(DeliveryAck{DeliveryID: "del-zero", Seq: 0})
			if err != nil {
				t.Fatalf("json.Marshal(DeliveryAck) error = %v", err)
			}
			if got, want := string(encoded), `{"delivery_id":"del-zero","seq":0}`; got != want {
				t.Fatalf("json.Marshal(DeliveryAck) = %s, want %s", got, want)
			}
		})
		normalized := NormalizeDeliveryAck(DeliveryAck{
			DeliveryID: " del-1 ", Seq: 2, RemoteMessageID: " remote-1 ",
			ReplaceRemoteMessageID: " remote-0 ",
		})
		if normalized.DeliveryID != "del-1" || normalized.Seq != 2 ||
			normalized.RemoteMessageID != "remote-1" || normalized.ReplaceRemoteMessageID != "remote-0" ||
			normalized.Outcome != DeliveryAckOutcomeSuccess {
			t.Fatalf("NormalizeDeliveryAck() = %#v, want canonical acknowledgement", normalized)
		}
		committed := NormalizeDeliveryAck(DeliveryAck{
			Outcome: " COMMITTED_RESULT_UNAVAILABLE ",
			Error:   &DeliveryErrorDetail{Message: " response omitted id "},
		})
		if committed.Outcome != DeliveryAckOutcomeCommittedResultUnavailable || committed.Error == nil ||
			committed.Error.Message != "response omitted id" {
			t.Fatalf("NormalizeDeliveryAck(committed) = %#v, want canonical committed failure", committed)
		}
		if got, want := strings.Join(
			DeliveryAckOutcomeValues(),
			",",
		), "success,committed_result_unavailable"; got != want {
			t.Fatalf("DeliveryAckOutcomeValues() = %q, want %q", got, want)
		}
	})
}

func TestDeliveryEventContract(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize delivery event wire payloads", func(t *testing.T) {
		t.Parallel()

		t.Run("Should canonicalize identity, type, content, and an omitted post operation", func(t *testing.T) {
			t.Parallel()

			normalized := NormalizeDeliveryEvent(DeliveryEvent{
				DeliveryID: " del-1 ", BridgeInstanceID: " brg-1 ",
				RoutingKey:     RoutingKey{Scope: " WORKSPACE ", WorkspaceID: " ws-1 ", BridgeInstanceID: " brg-1 "},
				DeliveryTarget: DeliveryTarget{BridgeInstanceID: " brg-1 ", PeerID: " peer-1 ", Mode: " REPLY "},
				EventType:      " DELTA ", Content: MessageContent{Text: " hello "},
			})
			if normalized.DeliveryID != "del-1" || normalized.BridgeInstanceID != "brg-1" ||
				normalized.EventType != DeliveryEventTypeDelta || normalized.Content.Text != "hello" ||
				normalized.Operation != DeliveryOperationPost || normalized.RoutingKey.WorkspaceID != "ws-1" ||
				normalized.DeliveryTarget.Mode != DeliveryModeReply {
				t.Fatalf("NormalizeDeliveryEvent() = %#v, want canonical event", normalized)
			}
		})

		t.Run("Should detach message references and provider metadata", func(t *testing.T) {
			t.Parallel()

			source := validDeliveryEvent(DeliveryEventTypeFinal)
			source.Final = true
			source.Operation = " EDIT "
			source.Reference = &DeliveryMessageReference{RemoteMessageID: " remote-1 "}
			source.ProviderMetadata = json.RawMessage(` {"provider":"slack"} `)
			normalized := NormalizeDeliveryEvent(source)
			if normalized.Reference == nil || normalized.Reference.RemoteMessageID != "remote-1" ||
				normalized.Operation != DeliveryOperationEdit || string(normalized.ProviderMetadata) != `{"provider":"slack"}` {
				t.Fatalf("NormalizeDeliveryEvent() = %#v, want canonical edit payload", normalized)
			}
			normalized.Reference.RemoteMessageID = "mutated"
			normalized.ProviderMetadata[0] = '['
			if got, want := source.Reference.RemoteMessageID, " remote-1 "; got != want {
				t.Fatalf("source.Reference.RemoteMessageID = %q after normalized mutation, want %q", got, want)
			}
			if got, want := string(source.ProviderMetadata), ` {"provider":"slack"} `; got != want {
				t.Fatalf("source.ProviderMetadata = %q after normalized mutation, want %q", got, want)
			}
		})

		t.Run("Should detach progress payloads", func(t *testing.T) {
			t.Parallel()

			source := validDeliveryEvent(DeliveryEventTypeProgress)
			source.Content = MessageContent{}
			source.Progress = validToolProgress()
			normalized := NormalizeDeliveryEvent(source)
			if normalized.Progress == nil || normalized.Progress.ToolCallID != "call-1" {
				t.Fatalf("NormalizeDeliveryEvent().Progress = %#v, want canonical progress", normalized.Progress)
			}
			normalized.Progress.Label = "mutated"
			if got, want := source.Progress.Label, "Running command"; got != want {
				t.Fatalf("source.Progress.Label = %q after normalized mutation, want %q", got, want)
			}
		})

		t.Run("Should preserve nil optional payloads", func(t *testing.T) {
			t.Parallel()

			normalized := NormalizeDeliveryEvent(DeliveryEvent{})
			if normalized.Reference != nil || normalized.Error != nil || normalized.Resume != nil ||
				normalized.Progress != nil || normalized.ProviderMetadata != nil {
				t.Fatalf("NormalizeDeliveryEvent(zero) = %#v, want nil optional payloads", normalized)
			}
		})
	})

	t.Run("Should validate each typed delivery family", func(t *testing.T) {
		t.Parallel()

		start := validDeliveryEvent(DeliveryEventTypeStart)
		delta := validDeliveryEvent(DeliveryEventTypeDelta)
		final := validDeliveryEvent(DeliveryEventTypeFinal)
		final.Final = true
		errorEvent := validDeliveryEvent(DeliveryEventTypeError)
		errorEvent.Final = true
		errorEvent.Error = &DeliveryErrorDetail{Message: "provider failed"}
		resume := validDeliveryEvent(DeliveryEventTypeResume)
		resume.Resume = &DeliveryResumeState{LatestEventType: DeliveryEventTypeDelta}
		deleteEvent := validDeliveryEvent(DeliveryEventTypeDelete)
		deleteEvent.Final = true
		deleteEvent.Operation = DeliveryOperationDelete
		deleteEvent.Reference = &DeliveryMessageReference{RemoteMessageID: "remote-1"}
		deleteEvent.Content = MessageContent{}
		progress := validDeliveryEvent(DeliveryEventTypeProgress)
		progress.Content = MessageContent{}
		progress.Progress = validToolProgress()

		for _, event := range []DeliveryEvent{start, delta, final, errorEvent, resume, deleteEvent, progress} {
			t.Run("Should validate a "+string(event.EventType)+" event", func(t *testing.T) {
				t.Parallel()

				if err := event.Validate(); err != nil {
					t.Fatalf("DeliveryEvent(%q).Validate() error = %v", event.EventType, err)
				}
			})
		}

		lifecycleCases := []struct {
			name  string
			event DeliveryEvent
			want  bool
		}{
			{
				name:  "Should classify an empty final with omitted operation as lifecycle-only",
				event: DeliveryEvent{EventType: DeliveryEventTypeFinal},
				want:  true,
			},
			{
				name:  "Should classify an empty final post as lifecycle-only",
				event: DeliveryEvent{EventType: DeliveryEventTypeFinal, Operation: DeliveryOperationPost},
				want:  true,
			},
			{
				name:  "Should reject a materialized final as lifecycle-only",
				event: DeliveryEvent{EventType: DeliveryEventTypeFinal, Content: MessageContent{Text: "answer"}},
			},
			{
				name:  "Should reject an empty final edit as lifecycle-only",
				event: DeliveryEvent{EventType: DeliveryEventTypeFinal, Operation: DeliveryOperationEdit},
			},
			{
				name:  "Should reject a progress event as lifecycle-only",
				event: DeliveryEvent{EventType: DeliveryEventTypeProgress},
			},
		}
		for _, test := range lifecycleCases {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				if got := test.event.IsLifecycleOnlyFinal(); got != test.want {
					t.Fatalf("IsLifecycleOnlyFinal() = %t, want %t", got, test.want)
				}
			})
		}

		t.Run("Should validate a final edit with a message reference", func(t *testing.T) {
			t.Parallel()

			event := validDeliveryEvent(DeliveryEventTypeFinal)
			event.Final = true
			event.Operation = DeliveryOperationEdit
			event.Reference = &DeliveryMessageReference{RemoteMessageID: "remote-1"}
			if err := event.Validate(); err != nil {
				t.Fatalf("DeliveryEvent(edit).Validate() error = %v", err)
			}
		})

		t.Run("Should validate an event without an optional delivery target", func(t *testing.T) {
			t.Parallel()

			event := validDeliveryEvent(DeliveryEventTypeDelta)
			event.DeliveryTarget = DeliveryTarget{}
			if err := event.Validate(); err != nil {
				t.Fatalf("DeliveryEvent(without target).Validate() error = %v", err)
			}
		})
	})

	t.Run("Should reject malformed identity, lifecycle, operation, and typed payload combinations", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			mutate func(*DeliveryEvent)
		}{
			{name: "Should require a delivery id", mutate: func(v *DeliveryEvent) { v.DeliveryID = "" }},
			{name: "Should require a bridge id", mutate: func(v *DeliveryEvent) { v.BridgeInstanceID = "" }},
			{name: "Should reject invalid routing", mutate: func(v *DeliveryEvent) { v.RoutingKey = RoutingKey{} }},
			{
				name:   "Should reject a routing bridge mismatch",
				mutate: func(v *DeliveryEvent) { v.RoutingKey.BridgeInstanceID = "other" },
			},
			{
				name:   "Should reject an invalid target",
				mutate: func(v *DeliveryEvent) { v.DeliveryTarget = DeliveryTarget{BridgeInstanceID: v.BridgeInstanceID} },
			},
			{
				name:   "Should reject a target bridge mismatch",
				mutate: func(v *DeliveryEvent) { v.DeliveryTarget.BridgeInstanceID = "other" },
			},
			{name: "Should reject a negative sequence", mutate: func(v *DeliveryEvent) { v.Seq = -1 }},
			{name: "Should reject an invalid operation", mutate: func(v *DeliveryEvent) { v.Operation = "replace" }},
			{
				name:   "Should reject a final start event",
				mutate: func(v *DeliveryEvent) { v.EventType = DeliveryEventTypeStart; v.Final = true },
			},
			{name: "Should reject a final delta event", mutate: func(v *DeliveryEvent) { v.Final = true }},
			{
				name:   "Should require final state on final events",
				mutate: func(v *DeliveryEvent) { v.EventType = DeliveryEventTypeFinal },
			},
			{name: "Should require final state on error events", mutate: func(v *DeliveryEvent) {
				v.EventType = DeliveryEventTypeError
				v.Error = &DeliveryErrorDetail{Message: "x"}
			}},
			{name: "Should require final state on delete events", mutate: func(v *DeliveryEvent) {
				v.EventType = DeliveryEventTypeDelete
				v.Operation = DeliveryOperationDelete
				v.Reference = &DeliveryMessageReference{DeliveryID: "del-0"}
				v.Content = MessageContent{}
			}},
			{name: "Should reject final state on progress events", mutate: func(v *DeliveryEvent) {
				v.EventType = DeliveryEventTypeProgress
				v.Final = true
				v.Content = MessageContent{}
				v.Progress = validToolProgress()
			}},
			{name: "Should require an event type", mutate: func(v *DeliveryEvent) { v.EventType = "" }},
			{name: "Should reject an unknown event type", mutate: func(v *DeliveryEvent) { v.EventType = "unknown" }},
			{
				name:   "Should reject invalid provider metadata",
				mutate: func(v *DeliveryEvent) { v.ProviderMetadata = json.RawMessage(`{`) },
			},
			{
				name:   "Should reject a post reference",
				mutate: func(v *DeliveryEvent) { v.Reference = &DeliveryMessageReference{DeliveryID: "del-0"} },
			},
			{
				name:   "Should require an edit reference",
				mutate: func(v *DeliveryEvent) { v.Operation = DeliveryOperationEdit },
			},
			{
				name:   "Should reject an empty edit reference",
				mutate: func(v *DeliveryEvent) { v.Operation = DeliveryOperationEdit; v.Reference = &DeliveryMessageReference{} },
			},
			{
				name:   "Should require delete operation on delete events",
				mutate: func(v *DeliveryEvent) { v.EventType = DeliveryEventTypeDelete; v.Final = true },
			},
			{name: "Should reject delete operation on delta events", mutate: func(v *DeliveryEvent) {
				v.Operation = DeliveryOperationDelete
				v.Reference = &DeliveryMessageReference{DeliveryID: "del-0"}
			}},
			{name: "Should reject edit operation on progress events", mutate: func(v *DeliveryEvent) {
				v.EventType = DeliveryEventTypeProgress
				v.Content = MessageContent{}
				v.Operation = DeliveryOperationEdit
				v.Reference = &DeliveryMessageReference{DeliveryID: "del-0"}
				v.Progress = validToolProgress()
			}},
			{
				name:   "Should require an error payload",
				mutate: func(v *DeliveryEvent) { v.EventType = DeliveryEventTypeError; v.Final = true },
			},
			{name: "Should reject an invalid error payload", mutate: func(v *DeliveryEvent) {
				v.EventType = DeliveryEventTypeError
				v.Final = true
				v.Error = &DeliveryErrorDetail{}
			}},
			{name: "Should reject resume data on error events", mutate: func(v *DeliveryEvent) {
				v.EventType = DeliveryEventTypeError
				v.Final = true
				v.Error = &DeliveryErrorDetail{Message: "x"}
				v.Resume = &DeliveryResumeState{LatestEventType: DeliveryEventTypeDelta}
			}},
			{
				name:   "Should require a resume payload",
				mutate: func(v *DeliveryEvent) { v.EventType = DeliveryEventTypeResume },
			},
			{name: "Should reject error data on resume events", mutate: func(v *DeliveryEvent) {
				v.EventType = DeliveryEventTypeResume
				v.Resume = &DeliveryResumeState{LatestEventType: DeliveryEventTypeDelta}
				v.Error = &DeliveryErrorDetail{Message: "x"}
			}},
			{name: "Should reject content on delete events", mutate: func(v *DeliveryEvent) {
				v.EventType = DeliveryEventTypeDelete
				v.Final = true
				v.Operation = DeliveryOperationDelete
				v.Reference = &DeliveryMessageReference{DeliveryID: "del-0"}
			}},
			{name: "Should reject typed payloads on delete events", mutate: func(v *DeliveryEvent) {
				v.EventType = DeliveryEventTypeDelete
				v.Final = true
				v.Operation = DeliveryOperationDelete
				v.Reference = &DeliveryMessageReference{DeliveryID: "del-0"}
				v.Content = MessageContent{}
				v.Error = &DeliveryErrorDetail{Message: "x"}
			}},
			{
				name:   "Should require a progress payload",
				mutate: func(v *DeliveryEvent) { v.EventType = DeliveryEventTypeProgress; v.Content = MessageContent{} },
			},
			{
				name:   "Should reject content on progress events",
				mutate: func(v *DeliveryEvent) { v.EventType = DeliveryEventTypeProgress; v.Progress = validToolProgress() },
			},
			{name: "Should reject error data on progress events", mutate: func(v *DeliveryEvent) {
				v.EventType = DeliveryEventTypeProgress
				v.Content = MessageContent{}
				v.Progress = validToolProgress()
				v.Error = &DeliveryErrorDetail{Message: "x"}
			}},
			{
				name:   "Should reject error data on delta events",
				mutate: func(v *DeliveryEvent) { v.Error = &DeliveryErrorDetail{Message: "x"} },
			},
			{
				name:   "Should reject resume data on delta events",
				mutate: func(v *DeliveryEvent) { v.Resume = &DeliveryResumeState{LatestEventType: DeliveryEventTypeDelta} },
			},
			{
				name:   "Should reject progress data on delta events",
				mutate: func(v *DeliveryEvent) { v.Progress = validToolProgress() },
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				event := validDeliveryEvent(DeliveryEventTypeDelta)
				test.mutate(&event)
				if err := event.Validate(); err == nil {
					t.Fatalf("DeliveryEvent.Validate(%s) error = nil", test.name)
				}
			})
		}
	})
}

func TestDeliveryRequestAndSnapshotContract(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize resumable delivery snapshots", func(t *testing.T) {
		t.Parallel()

		t.Run("Should canonicalize identity, routing, type, and omitted operation", func(t *testing.T) {
			t.Parallel()

			snapshot := validDeliverySnapshot()
			snapshot.DeliveryID = " del-1 "
			snapshot.SessionID = " sess-1 "
			snapshot.TurnID = " turn-1 "
			snapshot.BridgeInstanceID = " brg-1 "
			snapshot.LatestEventType = " DELTA "
			snapshot.Operation = ""
			snapshot.RemoteMessageID = " remote-1 "
			snapshot.ReplaceRemoteMessageID = " remote-0 "
			snapshot.Error = " retry later "
			normalized := NormalizeDeliverySnapshot(snapshot)
			if normalized.DeliveryID != "del-1" || normalized.SessionID != "sess-1" ||
				normalized.TurnID != "turn-1" || normalized.BridgeInstanceID != "brg-1" ||
				normalized.LatestEventType != DeliveryEventTypeDelta || normalized.Operation != DeliveryOperationPost ||
				normalized.RemoteMessageID != "remote-1" || normalized.ReplaceRemoteMessageID != "remote-0" ||
				normalized.Error != "retry later" {
				t.Fatalf("NormalizeDeliverySnapshot() = %#v, want canonical snapshot", normalized)
			}
		})

		t.Run("Should detach references and provider metadata", func(t *testing.T) {
			t.Parallel()

			source := validDeliverySnapshot()
			source.Operation = DeliveryOperationEdit
			source.Reference = &DeliveryMessageReference{RemoteMessageID: " remote-1 "}
			source.ProviderMetadata = json.RawMessage(` {"provider":"slack"} `)
			normalized := NormalizeDeliverySnapshot(source)
			if normalized.Reference == nil || normalized.Reference.RemoteMessageID != "remote-1" ||
				string(normalized.ProviderMetadata) != `{"provider":"slack"}` {
				t.Fatalf("NormalizeDeliverySnapshot() = %#v, want canonical owned payloads", normalized)
			}
			normalized.Reference.RemoteMessageID = "mutated"
			normalized.ProviderMetadata[0] = '['
			if got, want := source.Reference.RemoteMessageID, " remote-1 "; got != want {
				t.Fatalf("source.Reference.RemoteMessageID = %q after normalized mutation, want %q", got, want)
			}
			if got, want := string(source.ProviderMetadata), ` {"provider":"slack"} `; got != want {
				t.Fatalf("source.ProviderMetadata = %q after normalized mutation, want %q", got, want)
			}
		})

		t.Run("Should preserve nil optional payloads", func(t *testing.T) {
			t.Parallel()

			normalized := NormalizeDeliverySnapshot(DeliverySnapshot{})
			if normalized.Reference != nil || normalized.ProviderMetadata != nil {
				t.Fatalf("NormalizeDeliverySnapshot(zero) = %#v, want nil optional payloads", normalized)
			}
		})
	})

	t.Run("Should validate a matching resume request", func(t *testing.T) {
		t.Parallel()

		t.Run("Should accept a matching resume snapshot", func(t *testing.T) {
			t.Parallel()

			snapshot := validDeliverySnapshot()
			request := DeliveryRequest{Event: validDeliveryEvent(DeliveryEventTypeResume), Snapshot: &snapshot}
			request.Event.Resume = &DeliveryResumeState{LatestEventType: DeliveryEventTypeDelta}
			if err := request.Validate(); err != nil {
				t.Fatalf("DeliveryRequest.Validate() error = %v", err)
			}
		})

		t.Run("Should accept a non-resume event without a snapshot", func(t *testing.T) {
			t.Parallel()

			if err := (DeliveryRequest{Event: validDeliveryEvent(DeliveryEventTypeDelta)}).Validate(); err != nil {
				t.Fatalf("DeliveryRequest(non-resume).Validate() error = %v", err)
			}
		})
	})

	t.Run("Should reject invalid request and snapshot invariants", func(t *testing.T) {
		t.Parallel()

		snapshotMutations := []struct {
			name   string
			mutate func(*DeliverySnapshot)
		}{
			{name: "Should require a snapshot delivery id", mutate: func(v *DeliverySnapshot) { v.DeliveryID = "" }},
			{name: "Should require a snapshot session id", mutate: func(v *DeliverySnapshot) { v.SessionID = "" }},
			{name: "Should require a snapshot turn id", mutate: func(v *DeliverySnapshot) { v.TurnID = "" }},
			{
				name:   "Should require a snapshot bridge id",
				mutate: func(v *DeliverySnapshot) { v.BridgeInstanceID = "" },
			},
			{
				name:   "Should require a snapshot update timestamp",
				mutate: func(v *DeliverySnapshot) { v.UpdatedAt = time.Time{} },
			},
			{
				name:   "Should require valid snapshot routing",
				mutate: func(v *DeliverySnapshot) { v.RoutingKey = RoutingKey{} },
			},
			{
				name:   "Should reject a snapshot routing bridge mismatch",
				mutate: func(v *DeliverySnapshot) { v.RoutingKey.BridgeInstanceID = "other" },
			},
			{
				name:   "Should require a valid snapshot target",
				mutate: func(v *DeliverySnapshot) { v.DeliveryTarget = DeliveryTarget{} },
			},
			{
				name:   "Should reject a snapshot target bridge mismatch",
				mutate: func(v *DeliverySnapshot) { v.DeliveryTarget.BridgeInstanceID = "other" },
			},
			{name: "Should reject a negative latest sequence", mutate: func(v *DeliverySnapshot) { v.LatestSeq = -1 }},
			{
				name:   "Should reject a negative last-sent sequence",
				mutate: func(v *DeliverySnapshot) { v.LastSentSeq = -1 },
			},
			{
				name:   "Should reject a negative last-acked sequence",
				mutate: func(v *DeliverySnapshot) { v.LastAckedSeq = -1 },
			},
			{
				name:   "Should reject last-acked beyond last-sent",
				mutate: func(v *DeliverySnapshot) { v.LastAckedSeq = 3 },
			},
			{name: "Should reject last-sent beyond latest", mutate: func(v *DeliverySnapshot) { v.LastSentSeq = 3 }},
			{
				name:   "Should reject an unsupported snapshot operation",
				mutate: func(v *DeliverySnapshot) { v.Operation = "replace" },
			},
			{
				name:   "Should reject a reference on a post snapshot",
				mutate: func(v *DeliverySnapshot) { v.Reference = &DeliveryMessageReference{DeliveryID: "old"} },
			},
			{
				name:   "Should require a reference on an edit snapshot",
				mutate: func(v *DeliverySnapshot) { v.Operation = DeliveryOperationEdit },
			},
			{name: "Should reject an empty snapshot reference", mutate: func(v *DeliverySnapshot) {
				v.Operation = DeliveryOperationEdit
				v.Reference = &DeliveryMessageReference{}
			}},
			{
				name:   "Should reject invalid snapshot provider metadata",
				mutate: func(v *DeliverySnapshot) { v.ProviderMetadata = json.RawMessage(`{`) },
			},
			{
				name:   "Should require final state for a final snapshot event",
				mutate: func(v *DeliverySnapshot) { v.LatestEventType = DeliveryEventTypeFinal },
			},
		}
		for _, test := range snapshotMutations {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				snapshot := validDeliverySnapshot()
				test.mutate(&snapshot)
				if err := snapshot.Validate(); err == nil {
					t.Fatalf("DeliverySnapshot.Validate(%s) error = nil", test.name)
				}
			})
		}

		requestCases := []struct {
			name  string
			build func() DeliveryRequest
		}{
			{
				name: "Should require a snapshot on resume requests",
				build: func() DeliveryRequest {
					resume := validDeliveryEvent(DeliveryEventTypeResume)
					resume.Resume = &DeliveryResumeState{LatestEventType: DeliveryEventTypeDelta}
					return DeliveryRequest{Event: resume}
				},
			},
			{
				name: "Should reject snapshots on non-resume requests",
				build: func() DeliveryRequest {
					snapshot := validDeliverySnapshot()
					return DeliveryRequest{Event: validDeliveryEvent(DeliveryEventTypeDelta), Snapshot: &snapshot}
				},
			},
			{
				name: "Should reject an invalid resume snapshot",
				build: func() DeliveryRequest {
					resume := validDeliveryEvent(DeliveryEventTypeResume)
					resume.Resume = &DeliveryResumeState{LatestEventType: DeliveryEventTypeDelta}
					snapshot := validDeliverySnapshot()
					snapshot.SessionID = ""
					return DeliveryRequest{Event: resume, Snapshot: &snapshot}
				},
			},
			{
				name: "Should reject a resume snapshot delivery mismatch",
				build: func() DeliveryRequest {
					resume := validDeliveryEvent(DeliveryEventTypeResume)
					resume.Resume = &DeliveryResumeState{LatestEventType: DeliveryEventTypeDelta}
					snapshot := validDeliverySnapshot()
					snapshot.DeliveryID = "other"
					return DeliveryRequest{Event: resume, Snapshot: &snapshot}
				},
			},
			{
				name: "Should reject a resume snapshot bridge mismatch",
				build: func() DeliveryRequest {
					resume := validDeliveryEvent(DeliveryEventTypeResume)
					resume.Resume = &DeliveryResumeState{LatestEventType: DeliveryEventTypeDelta}
					snapshot := validDeliverySnapshot()
					snapshot.BridgeInstanceID = "other"
					snapshot.RoutingKey.BridgeInstanceID = "other"
					snapshot.DeliveryTarget.BridgeInstanceID = "other"
					return DeliveryRequest{Event: resume, Snapshot: &snapshot}
				},
			},
		}
		for _, test := range requestCases {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				if err := test.build().Validate(); err == nil {
					t.Fatalf("DeliveryRequest.Validate(%s) error = nil", test.name)
				}
			})
		}
	})
}

func validDeliveryEvent(eventType DeliveryEventType) DeliveryEvent {
	return DeliveryEvent{
		DeliveryID:       "del-1",
		BridgeInstanceID: "brg-1",
		RoutingKey: RoutingKey{
			Scope:            ScopeWorkspace,
			WorkspaceID:      "ws-1",
			BridgeInstanceID: "brg-1",
			PeerID:           "peer-1",
		},
		DeliveryTarget:   DeliveryTarget{BridgeInstanceID: "brg-1", PeerID: "peer-1", Mode: DeliveryModeReply},
		Seq:              2,
		EventType:        eventType,
		Content:          MessageContent{Text: "hello"},
		ProviderMetadata: json.RawMessage(`{"provider":"slack"}`),
	}
}

func validDeliverySnapshot() DeliverySnapshot {
	return DeliverySnapshot{
		DeliveryID:       "del-1",
		SessionID:        "sess-1",
		TurnID:           "turn-1",
		BridgeInstanceID: "brg-1",
		RoutingKey: RoutingKey{
			Scope:            ScopeWorkspace,
			WorkspaceID:      "ws-1",
			BridgeInstanceID: "brg-1",
			PeerID:           "peer-1",
		},
		DeliveryTarget:   DeliveryTarget{BridgeInstanceID: "brg-1", PeerID: "peer-1", Mode: DeliveryModeReply},
		LatestSeq:        2,
		LatestEventType:  DeliveryEventTypeDelta,
		CurrentContent:   MessageContent{Text: "hello"},
		Operation:        DeliveryOperationPost,
		ProviderMetadata: json.RawMessage(`{"provider":"slack"}`),
		LastSentSeq:      2,
		LastAckedSeq:     1,
		RemoteMessageID:  "remote-1",
		UpdatedAt:        time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC),
	}
}

func validToolProgress() *ToolProgress {
	return &ToolProgress{
		ToolCallID: "call-1", ToolID: "shell", Phase: ToolProgressPhaseStarted,
		Label: "Running command", Preview: "echo", Emoji: "🔧", Index: 1,
	}
}
