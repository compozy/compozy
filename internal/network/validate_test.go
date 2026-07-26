package network

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
)

func TestNormalizeEnvelopeValidKinds(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	opts := ValidateOptions{Now: now, MaxReplayAge: DefaultMaxReplayAge}

	target := "reviewer.sess-xyz"
	workID := "work_patch_42"
	replyTo := "msg_direct_00"
	traceID := "trace_patch_42"

	cases := []struct {
		name     string
		envelope Envelope
		wantKind Kind
		wantType reflect.Type
	}{
		{
			name: "Should normalize greet envelopes",
			envelope: Envelope{
				Protocol:    " agh-network/v0 ",
				WorkspaceID: " wks_test ",
				ID:          " msg_greet_01 ",
				Kind:        " greet ",
				Channel:     " builders ",
				From:        " coder.sess-abc ",
				To:          nil,
				TS:          now.Unix(),
				Body: mustRawJSON(t, map[string]any{
					"peer_card": map[string]any{
						"peer_id":               "coder.sess-abc",
						"profiles_supported":    []string{"agh-network/v0"},
						"capabilities":          []string{"workspace.patch.apply"},
						"artifacts_supported":   []string{"capability"},
						"trust_modes_supported": []string{"unverified"},
					},
					"summary": "hello",
				}),
			},
			wantKind: KindGreet,
			wantType: reflect.TypeFor[GreetBody](),
		},
		{
			name: "Should normalize whois response envelopes",
			envelope: Envelope{
				Protocol:    "agh-network/v0",
				WorkspaceID: testWorkspaceID,
				ID:          "msg_whois_01",
				Kind:        KindWhois,
				Channel:     "builders",
				From:        "coder.sess-abc",
				To:          &target,
				ReplyTo:     &replyTo,
				TS:          now.Unix(),
				Body: mustRawJSON(t, map[string]any{
					"type": "response",
					"peer_card": map[string]any{
						"peer_id":               "coder.sess-abc",
						"profiles_supported":    []string{"agh-network/v0"},
						"capabilities":          []string{"chat.translate"},
						"artifacts_supported":   []string{"capability"},
						"trust_modes_supported": []string{"unverified"},
					},
				}),
			},
			wantKind: KindWhois,
			wantType: reflect.TypeFor[WhoisBody](),
		},
		{
			name: "Should normalize say envelopes",
			envelope: Envelope{
				Protocol:    "agh-network/v0",
				WorkspaceID: testWorkspaceID,
				ID:          "msg_say_01",
				Kind:        KindSay,
				Channel:     "builders",
				Surface:     new(SurfaceThread),
				ThreadID:    new("thread_patch_42"),
				From:        "coder.sess-abc",
				WorkID:      new(workID),
				TS:          now.Unix(),
				Body: mustRawJSON(t, map[string]any{
					"text":   "working through it",
					"intent": "status_update",
				}),
			},
			wantKind: KindSay,
			wantType: reflect.TypeFor[SayBody](),
		},
		{
			name: "Should normalize direct envelopes",
			envelope: Envelope{
				Protocol:    "agh-network/v0",
				WorkspaceID: testWorkspaceID,
				ID:          "msg_direct_01",
				Kind:        KindSay,
				Channel:     "builders",
				Surface:     new(SurfaceDirect),
				DirectID:    new("direct_0123456789abcdef0123456789abcdef"),
				From:        "coder.sess-abc",
				To:          &target,
				WorkID:      &workID,
				TraceID:     &traceID,
				TS:          now.Unix(),
				Body: mustRawJSON(t, map[string]any{
					"text":   "please review auth.go",
					"intent": "review_request",
				}),
			},
			wantKind: KindSay,
			wantType: reflect.TypeFor[SayBody](),
		},
		{
			name: "Should normalize capability envelopes",
			envelope: Envelope{
				Protocol:    "agh-network/v0",
				WorkspaceID: testWorkspaceID,
				ID:          "msg_capability_01",
				Kind:        KindCapability,
				Channel:     "builders",
				Surface:     new(SurfaceThread),
				ThreadID:    new("thread_patch_42"),
				From:        "coder.sess-abc",
				WorkID:      new(workID),
				TS:          now.Unix(),
				Body: mustCapabilityBodyJSON(t, CapabilityEnvelopePayload{
					ID:               "review-fix",
					Summary:          "Review fix flow",
					Outcome:          "A reusable review fix workflow.",
					Version:          "1.0.0",
					ExecutionOutline: []string{"Inspect the issue", "Draft the fix"},
					Requirements:     []string{"workspace-write"},
				}),
			},
			wantKind: KindCapability,
			wantType: reflect.TypeFor[CapabilityBody](),
		},
		{
			name: "Should normalize receipt envelopes",
			envelope: Envelope{
				Protocol:    "agh-network/v0",
				WorkspaceID: testWorkspaceID,
				ID:          "msg_receipt_01",
				Kind:        KindReceipt,
				Channel:     "builders",
				Surface:     new(SurfaceDirect),
				DirectID:    new("direct_0123456789abcdef0123456789abcdef"),
				From:        "reviewer.sess-xyz",
				To:          new(target),
				WorkID:      new(workID),
				TS:          now.Unix(),
				Body: mustRawJSON(t, map[string]any{
					"for_id": "msg_direct_01",
					"status": "accepted",
					"detail": "Proceed.",
				}),
			},
			wantKind: KindReceipt,
			wantType: reflect.TypeFor[ReceiptBody](),
		},
		{
			name: "Should normalize trace envelopes",
			envelope: Envelope{
				Protocol:    "agh-network/v0",
				WorkspaceID: testWorkspaceID,
				ID:          "msg_trace_01",
				Kind:        KindTrace,
				Channel:     "builders",
				Surface:     new(SurfaceDirect),
				DirectID:    new("direct_0123456789abcdef0123456789abcdef"),
				From:        "reviewer.sess-xyz",
				To:          new("coder.sess-abc"),
				WorkID:      new(workID),
				TS:          now.Unix(),
				Body: mustRawJSON(t, map[string]any{
					"state":   "working",
					"message": "started",
				}),
			},
			wantKind: KindTrace,
			wantType: reflect.TypeFor[TraceBody](),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			normalized, err := NormalizeEnvelope(tc.envelope, opts)
			if err != nil {
				t.Fatalf("NormalizeEnvelope() error = %v", err)
			}

			if normalized.Kind != tc.wantKind {
				t.Fatalf("NormalizeEnvelope().Kind = %q, want %q", normalized.Kind, tc.wantKind)
			}
			if normalized.Channel != "builders" {
				t.Fatalf("NormalizeEnvelope().Channel = %q, want builders", normalized.Channel)
			}
			if normalized.From == "" || strings.Contains(normalized.From, " ") {
				t.Fatalf("NormalizeEnvelope().From = %q, want trimmed peer id", normalized.From)
			}

			body, err := normalized.DecodeBody()
			if err != nil {
				t.Fatalf("DecodeBody() error = %v", err)
			}
			if got := reflect.TypeOf(body); got != tc.wantType {
				t.Fatalf("DecodeBody() type = %v, want %v", got, tc.wantType)
			}
		})
	}
}

func TestParseEnvelopeRejectsInvalidFields(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	opts := ValidateOptions{Now: now, MaxReplayAge: DefaultMaxReplayAge}

	base := Envelope{
		Protocol:    ProtocolV0,
		WorkspaceID: testWorkspaceID,
		ID:          "msg_direct_01",
		Kind:        KindSay,
		Channel:     "builders",
		Surface:     new(SurfaceDirect),
		DirectID:    new("direct_0123456789abcdef0123456789abcdef"),
		From:        "coder.sess-abc",
		To:          new("reviewer.sess-xyz"),
		WorkID:      new("work_patch_42"),
		TS:          now.Unix(),
		Body: mustRawJSON(t, map[string]any{
			"text": "please review auth.go",
		}),
	}

	cases := []struct {
		name      string
		mutate    func(Envelope) Envelope
		wantErr   error
		wantMatch string
	}{
		{
			name: "Should reject protocol v1 envelopes",
			mutate: func(env Envelope) Envelope {
				env.Protocol = "agh-network/v1"
				return env
			},
			wantErr:   ErrInvalidField,
			wantMatch: "protocol",
		},
		{
			name: "Should reject legacy recipe kinds",
			mutate: func(env Envelope) Envelope {
				env.Kind = Kind("recipe")
				env.To = nil
				env.WorkID = nil
				env.Body = mustRawJSON(t, map[string]any{
					"recipe": map[string]any{
						"recipe_id": "review-fix",
					},
				})
				return env
			},
			wantErr:   ErrInvalidKind,
			wantMatch: `kind="recipe"`,
		},
		{
			name: "Should reject invalid channels",
			mutate: func(env Envelope) Envelope {
				env.Channel = "bad.channel"
				return env
			},
			wantErr:   ErrInvalidField,
			wantMatch: "channel",
		},
		{
			name: "Should reject invalid from peer IDs",
			mutate: func(env Envelope) Envelope {
				env.From = "BadPeer"
				return env
			},
			wantErr:   ErrInvalidField,
			wantMatch: "peer_id",
		},
		{
			name: "Should reject verified-format identities without proof",
			mutate: func(env Envelope) Envelope {
				env.From = "alice@39f713d0a644253f04529421b9f51b9b"
				return env
			},
			wantErr:   ErrInvalidField,
			wantMatch: "peer_id",
		},
		{
			name: "Should reject invalid destination peer IDs",
			mutate: func(env Envelope) Envelope {
				env.To = new("missing channel")
				return env
			},
			wantErr:   ErrInvalidField,
			wantMatch: "to",
		},
		{
			name: "Should reject missing work IDs",
			mutate: func(env Envelope) Envelope {
				env.Kind = KindReceipt
				env.WorkID = nil
				env.Body = mustRawJSON(t, map[string]any{
					"for_id": "msg_direct_01",
					"status": "accepted",
				})
				return env
			},
			wantErr:   ErrMissingField,
			wantMatch: "work_id",
		},
		{
			name: "Should reject whois responses without reply_to",
			mutate: func(env Envelope) Envelope {
				env.Kind = KindWhois
				env.Surface = nil
				env.ThreadID = nil
				env.DirectID = nil
				env.To = nil
				env.WorkID = nil
				env.Body = mustRawJSON(t, map[string]any{
					"type": "response",
					"peer_card": map[string]any{
						"peer_id":               "coder.sess-abc",
						"profiles_supported":    []string{"agh-network/v0"},
						"capabilities":          []string{"chat.translate"},
						"artifacts_supported":   []string{"capability"},
						"trust_modes_supported": []string{"unverified"},
					},
				})
				return env
			},
			wantErr:   ErrMissingField,
			wantMatch: "reply_to",
		},
		{
			name: "Should reject expired messages",
			mutate: func(env Envelope) Envelope {
				env.ExpiresAt = new(now.Add(-time.Second).Unix())
				return env
			},
			wantErr:   ErrExpired,
			wantMatch: "expires_at",
		},
		{
			name: "Should reject replay timestamps that are too old",
			mutate: func(env Envelope) Envelope {
				env.ExpiresAt = nil
				env.TS = now.Add(-10 * time.Minute).Unix()
				return env
			},
			wantErr:   ErrReplayTooOld,
			wantMatch: "max_replay_age",
		},
		{
			name: "Should reject future timestamps outside the replay window",
			mutate: func(env Envelope) Envelope {
				env.ExpiresAt = nil
				env.TS = now.Add(10 * time.Minute).Unix()
				return env
			},
			wantErr:   ErrReplayTooOld,
			wantMatch: "max_replay_age",
		},
		{
			name: "Should reject raw secrets in the body payload",
			mutate: func(env Envelope) Envelope {
				env.Body = mustRawJSON(t, map[string]any{
					"text":         "please review auth.go",
					"access_token": "provider-token",
				})
				return env
			},
			wantErr:   ErrInvalidBody,
			wantMatch: "raw secret material",
		},
		{
			name: "Should reject raw secrets in body keys",
			mutate: func(env Envelope) Envelope {
				env.Body = mustRawJSON(t, map[string]any{
					"agh_claim_secret-token": "",
					"text":                   "please review auth.go",
				})
				return env
			},
			wantErr:   ErrInvalidBody,
			wantMatch: "raw secret material",
		},
		{
			name: "Should reject raw secrets in extensions",
			mutate: func(env Envelope) Envelope {
				env.Ext = ExtensionMap{
					"agh.handoff": mustRawJSON(t, map[string]any{
						"note": "Bearer provider-token",
					}),
				}
				return env
			},
			wantErr:   ErrInvalidBody,
			wantMatch: "raw secret material",
		},
		{
			name: "Should reject raw claim tokens in extensions",
			mutate: func(env Envelope) Envelope {
				env.Ext = ExtensionMap{
					"agh.metadata": mustRawJSON(t, map[string]any{
						"claim_token": "agh_claim_NET05TOKEN123",
					}),
				}
				return env
			},
			wantErr:   ErrInvalidBody,
			wantMatch: "raw secret material",
		},
		{
			name: "Should reject raw secrets in proofs",
			mutate: func(env Envelope) Envelope {
				proof := Proof{
					"agh.proof": mustRawJSON(t, map[string]any{
						"access_token": "provider-token",
					}),
				}
				env.Proof = &proof
				return env
			},
			wantErr:   ErrInvalidBody,
			wantMatch: "network proof",
		},
		{
			name: "Should reject accepted receipts with reason codes",
			mutate: func(env Envelope) Envelope {
				env.Kind = KindReceipt
				env.From = "reviewer.sess-xyz"
				env.Body = mustRawJSON(t, map[string]any{
					"for_id":      "msg_direct_01",
					"status":      "accepted",
					"reason_code": "busy",
				})
				return env
			},
			wantErr:   ErrInvalidBody,
			wantMatch: "accepted receipt",
		},
		{
			name: "Should reject capability digest mismatches",
			mutate: func(env Envelope) Envelope {
				capability := canonicalCapabilityPayload(t, CapabilityEnvelopePayload{
					ID:               "review-fix",
					Summary:          "Review fix flow",
					Outcome:          "A reusable review fix workflow.",
					Version:          "1.0.0",
					ExecutionOutline: []string{"Inspect the issue", "Draft the fix"},
					Requirements:     []string{"workspace-write"},
				})
				capability.Digest = "sha256:not-the-canonical-digest"

				env.Kind = KindCapability
				env.To = nil
				env.Body = mustRawJSON(t, CapabilityBody{Capability: capability})
				return env
			},
			wantErr:   ErrVerificationFailed,
			wantMatch: "canonical digest",
		},
		{
			name: "Should reject capability bodies without nested capability objects",
			mutate: func(env Envelope) Envelope {
				env.Kind = KindCapability
				env.To = nil
				env.Body = mustRawJSON(t, map[string]any{
					"id":      "review-fix",
					"summary": "Review fix flow",
					"outcome": "A reusable review fix workflow.",
				})
				return env
			},
			wantErr:   ErrInvalidBody,
			wantMatch: `{"capability":{...}}`,
		},
		{
			name: "Should reject capabilities without outcomes",
			mutate: func(env Envelope) Envelope {
				env.Kind = KindCapability
				env.To = nil
				env.Body = mustRawJSON(t, map[string]any{
					"capability": map[string]any{
						"id":      "review-fix",
						"summary": "Review fix flow",
						"digest":  "sha256:missing-outcome",
					},
				})
				return env
			},
			wantErr:   ErrInvalidBody,
			wantMatch: "capability.outcome is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := NormalizeEnvelope(tc.mutate(base), opts)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("NormalizeEnvelope() error = %v, want %v", err, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantMatch) {
				t.Fatalf("NormalizeEnvelope() error = %v, want substring %q", err, tc.wantMatch)
			}
		})
	}
}

func TestValidateEnvelopeConversationContainerInvariants(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	opts := ValidateOptions{Now: now, MaxReplayAge: DefaultMaxReplayAge}
	base := Envelope{
		Protocol:    ProtocolV0,
		WorkspaceID: testWorkspaceID,
		ID:          "msg_surface_01",
		Kind:        KindSay,
		Channel:     "builders",
		Surface:     new(SurfaceThread),
		ThreadID:    new("thread_patch_42"),
		From:        "coder.sess-abc",
		TS:          now.Unix(),
		Body:        mustRawJSON(t, SayBody{Text: "thread update"}),
	}

	cases := []struct {
		name      string
		mutate    func(Envelope) Envelope
		wantErr   error
		wantMatch string
	}{
		{
			name: "Should accept thread surface with thread_id",
			mutate: func(env Envelope) Envelope {
				return env
			},
		},
		{
			name: "Should accept direct surface with direct_id",
			mutate: func(env Envelope) Envelope {
				env.Surface = new(SurfaceDirect)
				env.ThreadID = nil
				env.DirectID = new("direct_0123456789abcdef0123456789abcdef")
				env.To = new("reviewer.sess-xyz")
				env.WorkID = new("work_patch_42")
				return env
			},
		},
		{
			name: "Should reject thread surface without thread_id",
			mutate: func(env Envelope) Envelope {
				env.ThreadID = nil
				return env
			},
			wantErr:   ErrMissingField,
			wantMatch: "thread_id",
		},
		{
			name: "Should reject thread surface with direct_id",
			mutate: func(env Envelope) Envelope {
				env.DirectID = new("direct_0123456789abcdef0123456789abcdef")
				return env
			},
			wantErr:   ErrInvalidField,
			wantMatch: "direct_id",
		},
		{
			name: "Should reject direct surface without direct_id",
			mutate: func(env Envelope) Envelope {
				env.Surface = new(SurfaceDirect)
				env.ThreadID = nil
				return env
			},
			wantErr:   ErrMissingField,
			wantMatch: "direct_id",
		},
		{
			name: "Should reject direct surface with thread_id",
			mutate: func(env Envelope) Envelope {
				env.Surface = new(SurfaceDirect)
				env.DirectID = new("direct_0123456789abcdef0123456789abcdef")
				return env
			},
			wantErr:   ErrInvalidField,
			wantMatch: "thread_id",
		},
		{
			name: "Should reject thread_id without surface",
			mutate: func(env Envelope) Envelope {
				env.Surface = nil
				return env
			},
			wantErr:   ErrInvalidField,
			wantMatch: "thread_id requires surface",
		},
		{
			name: "Should reject direct_id without surface",
			mutate: func(env Envelope) Envelope {
				env.Surface = nil
				env.ThreadID = nil
				env.DirectID = new("direct_0123456789abcdef0123456789abcdef")
				return env
			},
			wantErr:   ErrInvalidField,
			wantMatch: "direct_id requires surface",
		},
		{
			name: "Should reject conversation kind without surface",
			mutate: func(env Envelope) Envelope {
				env.Surface = nil
				env.ThreadID = nil
				return env
			},
			wantErr:   ErrMissingField,
			wantMatch: "surface",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := NormalizeEnvelope(tc.mutate(base), opts)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("NormalizeEnvelope() error = %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("NormalizeEnvelope() error = %v, want %v", err, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantMatch) {
				t.Fatalf("NormalizeEnvelope() error = %v, want substring %q", err, tc.wantMatch)
			}
		})
	}
}

func TestValidateEnvelopeDiscoveryKindsRejectConversationFields(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	opts := ValidateOptions{Now: now, MaxReplayAge: DefaultMaxReplayAge}
	base := Envelope{
		Protocol:    ProtocolV0,
		WorkspaceID: testWorkspaceID,
		ID:          "msg_greet_01",
		Kind:        KindGreet,
		Channel:     "builders",
		From:        "coder.sess-abc",
		TS:          now.Unix(),
		Body: mustRawJSON(t, GreetBody{
			PeerCard: mustPeerCard(t, "coder.sess-abc"),
		}),
	}

	cases := []struct {
		name      string
		kind      Kind
		body      json.RawMessage
		mutate    func(Envelope) Envelope
		wantMatch string
	}{
		{
			name: "Should reject greet with surface",
			kind: KindGreet,
			mutate: func(env Envelope) Envelope {
				env.Surface = new(SurfaceThread)
				return env
			},
			wantMatch: "surface",
		},
		{
			name: "Should reject greet with thread_id",
			kind: KindGreet,
			mutate: func(env Envelope) Envelope {
				env.ThreadID = new("thread_patch_42")
				return env
			},
			wantMatch: "thread_id",
		},
		{
			name: "Should reject greet with direct_id",
			kind: KindGreet,
			mutate: func(env Envelope) Envelope {
				env.DirectID = new("direct_0123456789abcdef0123456789abcdef")
				return env
			},
			wantMatch: "direct_id",
		},
		{
			name: "Should reject greet with work_id",
			kind: KindGreet,
			mutate: func(env Envelope) Envelope {
				env.WorkID = new("work_patch_42")
				return env
			},
			wantMatch: "work_id",
		},
		{
			name: "Should reject whois with surface",
			kind: KindWhois,
			body: mustRawJSON(t, WhoisBody{
				Type: WhoisTypeRequest,
			}),
			mutate: func(env Envelope) Envelope {
				env.Surface = new(SurfaceThread)
				return env
			},
			wantMatch: "surface",
		},
		{
			name: "Should reject whois with thread_id",
			kind: KindWhois,
			body: mustRawJSON(t, WhoisBody{
				Type: WhoisTypeRequest,
			}),
			mutate: func(env Envelope) Envelope {
				env.ThreadID = new("thread_patch_42")
				return env
			},
			wantMatch: "thread_id",
		},
		{
			name: "Should reject whois with direct_id",
			kind: KindWhois,
			body: mustRawJSON(t, WhoisBody{
				Type: WhoisTypeRequest,
			}),
			mutate: func(env Envelope) Envelope {
				env.DirectID = new("direct_0123456789abcdef0123456789abcdef")
				return env
			},
			wantMatch: "direct_id",
		},
		{
			name: "Should reject whois with work_id",
			kind: KindWhois,
			body: mustRawJSON(t, WhoisBody{
				Type: WhoisTypeRequest,
			}),
			mutate: func(env Envelope) Envelope {
				env.WorkID = new("work_patch_42")
				return env
			},
			wantMatch: "work_id",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			env := base
			env.Kind = tc.kind
			if tc.body != nil {
				env.Body = tc.body
			}
			_, err := NormalizeEnvelope(tc.mutate(env), opts)
			if !errors.Is(err, ErrInvalidField) {
				t.Fatalf("NormalizeEnvelope() error = %v, want ErrInvalidField", err)
			}
			if !strings.Contains(err.Error(), tc.wantMatch) {
				t.Fatalf("NormalizeEnvelope() error = %v, want substring %q", err, tc.wantMatch)
			}
		})
	}
}

func TestParseEnvelopeRejectsLegacyConversationFields(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	opts := ValidateOptions{Now: now, MaxReplayAge: DefaultMaxReplayAge}

	cases := []struct {
		name    string
		raw     []byte
		wantErr error
	}{
		{
			name: "Should reject legacy interaction_id",
			raw: []byte(`{
			  "protocol": "agh-network/v0",
			  "workspace_id": "wks_test",
			  "id": "msg_legacy_interaction",
			  "kind": "say",
			  "channel": "builders",
			  "surface": "direct",
			  "direct_id": "direct_0123456789abcdef0123456789abcdef",
			  "from": "coder.sess-abc",
			  "to": "reviewer.sess-xyz",
			  "interaction_id": "work_patch_42",
			  "ts": 1775822400,
			  "body": {"text": "legacy"}
			}`),
			wantErr: ErrLegacyFieldRejected,
		},
		{
			name: "Should reject legacy direct kind",
			raw: []byte(`{
			  "protocol": "agh-network/v0",
			  "workspace_id": "wks_test",
			  "id": "msg_legacy_direct_kind",
			  "kind": "direct",
			  "channel": "builders",
			  "surface": "direct",
			  "direct_id": "direct_0123456789abcdef0123456789abcdef",
			  "from": "coder.sess-abc",
			  "to": "reviewer.sess-xyz",
			  "work_id": "work_patch_42",
			  "ts": 1775822400,
			  "body": {"text": "legacy"}
			}`),
			wantErr: ErrInvalidKind,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseEnvelope(tc.raw, opts)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ParseEnvelope() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateEnvelopeRequiresWorkIDForLifecycleKinds(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	opts := ValidateOptions{Now: now, MaxReplayAge: DefaultMaxReplayAge}

	cases := []struct {
		name string
		env  Envelope
	}{
		{
			name: "Should require work_id for capability",
			env: Envelope{
				Protocol:    ProtocolV0,
				WorkspaceID: testWorkspaceID,
				ID:          "msg_capability_missing_work",
				Kind:        KindCapability,
				Channel:     "builders",
				Surface:     new(SurfaceDirect),
				DirectID:    new("direct_0123456789abcdef0123456789abcdef"),
				From:        "reviewer.sess-xyz",
				To:          new("coder.sess-abc"),
				TS:          now.Unix(),
				Body: mustCapabilityBodyJSON(t, CapabilityEnvelopePayload{
					ID:      "capability-review",
					Summary: "Review capability",
					Outcome: "Patch reviewed",
				}),
			},
		},
		{
			name: "Should require work_id for receipt",
			env: Envelope{
				Protocol:    ProtocolV0,
				WorkspaceID: testWorkspaceID,
				ID:          "msg_receipt_missing_work",
				Kind:        KindReceipt,
				Channel:     "builders",
				Surface:     new(SurfaceDirect),
				DirectID:    new("direct_0123456789abcdef0123456789abcdef"),
				From:        "reviewer.sess-xyz",
				To:          new("coder.sess-abc"),
				TS:          now.Unix(),
				Body: mustRawJSON(t, ReceiptBody{
					ForID:  "msg_direct_01",
					Status: ReceiptStatusAccepted,
				}),
			},
		},
		{
			name: "Should require work_id for trace",
			env: Envelope{
				Protocol:    ProtocolV0,
				WorkspaceID: testWorkspaceID,
				ID:          "msg_trace_missing_work",
				Kind:        KindTrace,
				Channel:     "builders",
				Surface:     new(SurfaceDirect),
				DirectID:    new("direct_0123456789abcdef0123456789abcdef"),
				From:        "reviewer.sess-xyz",
				To:          new("coder.sess-abc"),
				TS:          now.Unix(),
				Body: mustRawJSON(t, TraceBody{
					State: WorkStateWorking,
				}),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := NormalizeEnvelope(tc.env, opts)
			if !errors.Is(err, ErrMissingField) {
				t.Fatalf("NormalizeEnvelope() error = %v, want ErrMissingField", err)
			}
			if !strings.Contains(err.Error(), "work_id") {
				t.Fatalf("NormalizeEnvelope() error = %v, want work_id", err)
			}
		})
	}
}

func TestRFC004SignedContentConversationFieldsAffectCanonicalBytes(t *testing.T) {
	t.Parallel()

	base := Envelope{
		Protocol:    ProtocolV0,
		WorkspaceID: testWorkspaceID,
		ID:          "msg_signed_01",
		Kind:        KindSay,
		Channel:     "builders",
		From:        "coder.sess-abc",
		TS:          time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC).Unix(),
		Body:        mustRawJSON(t, SayBody{Text: "signed content"}),
	}
	baseBytes := mustMarshalEnvelopeBytes(t, base)

	presentZero := base
	zeroSurface := Surface("")
	presentZero.Surface = &zeroSurface
	presentZero.ThreadID = new("")
	presentZero.DirectID = new("")
	presentZero.WorkID = new("")
	if bytes.Equal(baseBytes, mustMarshalEnvelopeBytes(t, presentZero)) {
		t.Fatal("canonical bytes did not distinguish absent nullable conversation fields from present zero values")
	}

	cases := []struct {
		name   string
		mutate func(Envelope) Envelope
	}{
		{
			name: "surface",
			mutate: func(env Envelope) Envelope {
				env.Surface = new(SurfaceThread)
				return env
			},
		},
		{
			name: "thread_id",
			mutate: func(env Envelope) Envelope {
				env.Surface = new(SurfaceThread)
				env.ThreadID = new("thread_signed_a")
				return env
			},
		},
		{
			name: "direct_id",
			mutate: func(env Envelope) Envelope {
				env.Surface = new(SurfaceDirect)
				env.DirectID = new("direct_0123456789abcdef0123456789abcdef")
				return env
			},
		},
		{
			name: "work_id",
			mutate: func(env Envelope) Envelope {
				env.WorkID = new("work_signed_a")
				return env
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			changed := mustMarshalEnvelopeBytes(t, tc.mutate(base))
			if bytes.Equal(baseBytes, changed) {
				t.Fatalf("canonical bytes unchanged after %s changed", tc.name)
			}
		})
	}
}

func TestNormalizeEnvelopeAllowsWhitespaceOnlyStrings(t *testing.T) {
	t.Run("Should allow whitespace-only optional fields", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
		opts := ValidateOptions{Now: now, MaxReplayAge: DefaultMaxReplayAge}

		envelope := Envelope{
			Protocol:    "agh-network/v0",
			WorkspaceID: testWorkspaceID,
			ID:          "msg_say_whitespace_01",
			Kind:        KindSay,
			Channel:     "builders",
			Surface:     new(SurfaceThread),
			ThreadID:    new("thread_patch_42"),
			From:        "coder.sess-abc",
			TS:          now.Unix(),
			Body: mustRawJSON(t, map[string]any{
				"text":    "progress update",
				"summary": "   ",
			}),
		}

		if _, err := NormalizeEnvelope(envelope, opts); err != nil {
			t.Fatalf("NormalizeEnvelope(whitespace-only optional field) error = %v", err)
		}
	})
}

func TestExtRoundTripPreservesOpaqueKeys(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve opaque ext keys on round trip", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
		raw := []byte(`{
	  "protocol": "agh-network/v0",
	  "workspace_id": "wks_test",
	  "id": "msg_direct_ext_01",
		  "kind": "say",
		  "channel": "builders",
		  "surface": "direct",
		  "direct_id": "direct_0123456789abcdef0123456789abcdef",
		  "from": "coder.sess-abc",
		  "to": "reviewer.sess-xyz",
		  "work_id": "work_patch_42",
	  "ts": 1775822400,
	  "body": {"text": "review this"},
	  "proof": {"profile": "agh-network.trust.ed25519-jcs/v1"},
	  "ext": {
	    "unknown.vendor": {"nested": [1, true, "x"]},
	    "agh.workflow": {"ticket": "NET-42"},
	    "agh.handoff": {"turn": 3},
	    "agh.capability_catalog": {
	      "capabilities": [
	        {"id": "review-pr", "summary": "Review pull requests", "outcome": "Actionable review findings"}
	      ]
	    }
	  }
	}`)

		env, err := ParseEnvelope(raw, ValidateOptions{Now: now})
		if err != nil {
			t.Fatalf("ParseEnvelope() error = %v", err)
		}
		if env.Proof == nil {
			t.Fatalf("ParseEnvelope().Proof = nil, want preserved proof payload")
		}

		encoded, err := json.Marshal(env)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}

		roundTripped, err := ParseEnvelope(encoded, ValidateOptions{Now: now})
		if err != nil {
			t.Fatalf("ParseEnvelope(round trip) error = %v", err)
		}

		if !reflect.DeepEqual(extSnapshot(env.Ext), extSnapshot(roundTripped.Ext)) {
			t.Fatalf("Ext round-trip mismatch = %#v, want %#v", extSnapshot(roundTripped.Ext), extSnapshot(env.Ext))
		}
	})
}

func mustRawJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(%T) error = %v", value, err)
	}
	return json.RawMessage(data)
}

func mustMarshalEnvelopeBytes(t *testing.T, env Envelope) []byte {
	t.Helper()

	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("json.Marshal(Envelope) error = %v", err)
	}
	return data
}

func canonicalCapabilityPayload(t *testing.T, capability CapabilityEnvelopePayload) CapabilityEnvelopePayload {
	t.Helper()

	if strings.TrimSpace(capability.Digest) != "" {
		capability.Digest = strings.TrimSpace(capability.Digest)
		return capability
	}

	digest, err := aghconfig.CanonicalCapabilityDigest(aghconfig.CapabilityDef{
		ID:                capability.ID,
		Summary:           capability.Summary,
		Outcome:           capability.Outcome,
		Version:           capability.Version,
		ContextNeeded:     append([]string(nil), capability.ContextNeeded...),
		ArtifactsExpected: append([]string(nil), capability.ArtifactsExpected...),
		ExecutionOutline:  append([]string(nil), capability.ExecutionOutline...),
		Constraints:       append([]string(nil), capability.Constraints...),
		Examples:          append([]string(nil), capability.Examples...),
		Requirements:      append([]string(nil), capability.Requirements...),
	})
	if err != nil {
		t.Fatalf("CanonicalCapabilityDigest() error = %v", err)
	}

	capability.Digest = digest
	return capability
}

func mustCapabilityBodyJSON(t *testing.T, capability CapabilityEnvelopePayload) json.RawMessage {
	t.Helper()

	return mustRawJSON(t, CapabilityBody{Capability: canonicalCapabilityPayload(t, capability)})
}

func testDirectRef() ConversationRef {
	return ConversationRef{
		WorkspaceID: testWorkspaceID,
		Channel:     "builders",
		Surface:     SurfaceDirect,
		DirectID:    "direct_0123456789abcdef0123456789abcdef",
	}
}

func withDirectSurface(env Envelope) Envelope {
	if isConversationKind(env.Kind) && env.Surface == nil {
		env.Surface = new(SurfaceDirect)
		env.DirectID = new(testDirectRef().DirectID)
	}
	return env
}

func extSnapshot(ext ExtensionMap) map[string]any {
	if len(ext) == 0 {
		return map[string]any{}
	}

	snapshot := make(map[string]any, len(ext))
	for key, value := range ext {
		var decoded any
		if err := json.Unmarshal(value, &decoded); err != nil {
			snapshot[key] = string(value)
			continue
		}
		snapshot[key] = decoded
	}
	return snapshot
}
