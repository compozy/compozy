package network

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

const testWorkspaceID = "wks_test"

func TestPreviewTextForRawBodyShouldBoundUTF8(t *testing.T) {
	t.Parallel()

	const wantRunes = 160
	preview := PreviewTextForRawBody(
		KindSay,
		mustRawJSON(t, SayBody{Text: strings.Repeat("界", wantRunes+20)}),
	)
	if !utf8.ValidString(preview) {
		t.Fatalf("PreviewTextForRawBody() = %q, want valid UTF-8", preview)
	}
	if got := utf8.RuneCountInString(preview); got != wantRunes {
		t.Fatalf("PreviewTextForRawBody() runes = %d, want %d", got, wantRunes)
	}
}

func TestPreviewForBodyVariants(t *testing.T) {
	t.Parallel()

	detail := "receipt detail"
	tests := []struct {
		name string
		body Body
		want string
	}{
		{name: "use a greet summary", body: GreetBody{Summary: "hello"}, want: "hello"},
		{name: "use a whois request query", body: WhoisBody{Type: WhoisTypeRequest, Query: "review"}, want: "review"},
		{name: "hide a whois response query", body: WhoisBody{Type: WhoisTypeResponse, Query: "review"}, want: ""},
		{name: "use say text", body: SayBody{Text: "broadcast"}, want: "broadcast"},
		{
			name: "prefer a capability summary",
			body: CapabilityBody{Capability: CapabilityEnvelopePayload{
				ID: "capability-id", Summary: "summary", Outcome: "outcome",
			}},
			want: "summary",
		},
		{
			name: "fall back to a capability outcome",
			body: CapabilityBody{Capability: CapabilityEnvelopePayload{
				ID: "capability-id", Outcome: "outcome",
			}},
			want: "outcome",
		},
		{
			name: "fall back to a capability id",
			body: CapabilityBody{Capability: CapabilityEnvelopePayload{ID: "capability-id"}},
			want: "capability-id",
		},
		{name: "use receipt detail", body: ReceiptBody{Detail: &detail}, want: "receipt detail"},
		{name: "leave an empty receipt blank", body: ReceiptBody{}, want: ""},
		{name: "use a trace message", body: TraceBody{Message: "working"}, want: "working"},
		{name: "leave an empty trace blank", body: TraceBody{}, want: ""},
	}
	for _, test := range tests {
		t.Run("Should "+test.name, func(t *testing.T) {
			t.Parallel()
			if got := previewForBody(test.body); got != test.want {
				t.Fatalf("previewForBody() = %q, want %q", got, test.want)
			}
		})
	}

	t.Run("Should decode a valid public body", func(t *testing.T) {
		t.Parallel()
		if got := PreviewTextForRawBody(KindSay, mustRawJSON(t, SayBody{Text: "safe"})); got != "safe" {
			t.Fatalf("PreviewTextForRawBody() = %q, want safe", got)
		}
	})
	for _, test := range []struct {
		name string
		kind Kind
		raw  json.RawMessage
	}{
		{name: "reject malformed JSON", kind: KindSay, raw: json.RawMessage(`{"text":`)},
		{name: "reject a body from another kind", kind: KindSay, raw: json.RawMessage(`{"summary":"wrong"}`)},
		{name: "reject an unknown kind", kind: Kind("unknown"), raw: json.RawMessage(`{}`)},
	} {
		t.Run("Should "+test.name, func(t *testing.T) {
			t.Parallel()
			if got := PreviewTextForRawBody(test.kind, test.raw); got != "" {
				t.Fatalf("PreviewTextForRawBody() = %q, want empty", got)
			}
		})
	}
}

func TestResolveGreetSummaryUsesDeterministicFallbacks(t *testing.T) {
	t.Parallel()

	displayName := "Review Agent"
	tests := []struct {
		name    string
		card    PeerCard
		summary string
		want    string
	}{
		{
			name: "prefer an explicit summary",
			card: PeerCard{PeerID: "reviewer.sess-a"}, summary: "  Custom presence  ",
			want: "Custom presence",
		},
		{
			name: "use display name and rich capability brief",
			card: PeerCard{
				PeerID: "reviewer.sess-a", DisplayName: &displayName,
				Ext: ExtensionMap{capabilityBriefExtKey: mustRawJSON(t, []capabilityBrief{
					{ID: "review-pr", Summary: "Review pull requests"},
					{ID: "draft-spec", Summary: "Draft specifications"},
				})},
			},
			want: "Review Agent ready for Review pull requests +1 more",
		},
		{
			name: "fall back from an empty brief summary to its id",
			card: PeerCard{
				PeerID: "reviewer.sess-a",
				Ext: ExtensionMap{capabilityBriefExtKey: mustRawJSON(t, []capabilityBrief{{
					ID: "review-pr",
				}})},
			},
			want: "reviewer.sess-a ready for review-pr",
		},
		{
			name: "ignore malformed briefs and use declared capabilities",
			card: PeerCard{
				PeerID: "reviewer.sess-a", Capabilities: []string{"", "review-pr", "draft-spec"},
				Ext: ExtensionMap{capabilityBriefExtKey: json.RawMessage(`{`)},
			},
			want: "reviewer.sess-a ready for review-pr +1 more",
		},
		{
			name: "report plain presence when no capability is available",
			card: PeerCard{}, want: "Peer is present",
		},
	}
	for _, test := range tests {
		t.Run("Should "+test.name, func(t *testing.T) {
			t.Parallel()
			if got := ResolveGreetSummary(test.card, test.summary); got != test.want {
				t.Fatalf("ResolveGreetSummary() = %q, want %q", got, test.want)
			}
		})
	}

	t.Run("Should ignore empty capability brief entries", func(t *testing.T) {
		t.Parallel()
		raw := mustRawJSON(t, []capabilityBrief{{}, {ID: "review-pr", Summary: " Review "}})
		if got := decodeCapabilityBriefs(raw); len(got) != 1 ||
			got[0].ID != "review-pr" || got[0].Summary != "Review" {
			t.Fatalf("decodeCapabilityBriefs() = %#v, want one normalized brief", got)
		}
	})
}

func TestConversationRefHelpersExposeValidatedContainerIdentity(t *testing.T) {
	t.Parallel()

	thread := ConversationRef{
		WorkspaceID: testWorkspaceID, Channel: "builders", Surface: SurfaceThread,
		ThreadID: "thread_helpers",
	}
	if err := ValidateConversationRef(thread); err != nil {
		t.Fatalf("ValidateConversationRef(thread) error = %v", err)
	}
	if !thread.IsThread() || thread.IsDirect() {
		t.Fatalf("thread helpers = IsThread:%t IsDirect:%t", thread.IsThread(), thread.IsDirect())
	}
	if got, want := thread.ContainerKey(), testWorkspaceID+"\x00builders\x00thread\x00thread_helpers"; got != want {
		t.Fatalf("thread ContainerKey() = %q, want %q", got, want)
	}

	direct := ConversationRef{
		WorkspaceID: testWorkspaceID, Channel: "builders", Surface: SurfaceDirect,
		DirectID: "direct_0123456789abcdef0123456789abcdef",
	}
	if err := ValidateConversationRef(direct); err != nil {
		t.Fatalf("ValidateConversationRef(direct) error = %v", err)
	}
	if !direct.IsDirect() || direct.IsThread() {
		t.Fatalf("direct helpers = IsThread:%t IsDirect:%t", direct.IsThread(), direct.IsDirect())
	}
	if err := ValidateSurface(SurfaceThread); err != nil {
		t.Fatalf("ValidateSurface(thread) error = %v", err)
	}
	if err := ValidateSurface(Surface("unknown")); !errors.Is(err, ErrInvalidField) {
		t.Fatalf("ValidateSurface(unknown) error = %v, want ErrInvalidField", err)
	}
}

func TestEnumValidationAndBodyKindHelpers(t *testing.T) {
	t.Parallel()

	validKinds := []Kind{KindGreet, KindWhois, KindSay, KindCapability, KindReceipt, KindTrace}
	for _, kind := range validKinds {
		t.Run("ShouldValidateKnownKind"+string(kind), func(t *testing.T) {
			t.Parallel()

			if err := kind.Validate(); err != nil {
				t.Fatalf("Kind(%q).Validate() error = %v", kind, err)
			}
		})
	}
	if err := Kind("invalid").Validate(); !errors.Is(err, ErrInvalidKind) {
		t.Fatalf("Kind(invalid).Validate() error = %v, want ErrInvalidKind", err)
	}

	validReceiptStatuses := []ReceiptStatus{
		ReceiptStatusAccepted,
		ReceiptStatusRejected,
		ReceiptStatusDuplicate,
		ReceiptStatusExpired,
		ReceiptStatusUnsupported,
		ReceiptStatusCanceled,
	}
	for _, status := range validReceiptStatuses {
		t.Run("ShouldValidateKnownReceiptStatus"+string(status), func(t *testing.T) {
			t.Parallel()

			if err := status.Validate(); err != nil {
				t.Fatalf("ReceiptStatus(%q).Validate() error = %v", status, err)
			}
		})
	}
	if err := ReceiptStatus("unknown").Validate(); !errors.Is(err, ErrInvalidField) {
		t.Fatalf("ReceiptStatus(unknown).Validate() error = %v, want ErrInvalidField", err)
	}

	if err := WhoisTypeRequest.Validate(); err != nil {
		t.Fatalf("WhoisTypeRequest.Validate() error = %v", err)
	}
	if err := WhoisType("other").Validate(); !errors.Is(err, ErrInvalidField) {
		t.Fatalf("WhoisType(other).Validate() error = %v, want ErrInvalidField", err)
	}

	if err := ReasonCodeBusy.Validate(); err != nil {
		t.Fatalf("ReasonCodeBusy.Validate() error = %v", err)
	}
	if err := ReasonCode("mystery").Validate(); !errors.Is(err, ErrInvalidField) {
		t.Fatalf("ReasonCode(mystery).Validate() error = %v, want ErrInvalidField", err)
	}

	if err := WorkStateWorking.Validate(); err != nil {
		t.Fatalf("WorkStateWorking.Validate() error = %v", err)
	}
	if err := WorkState("drifting").Validate(); !errors.Is(err, ErrInvalidField) {
		t.Fatalf("WorkState(drifting).Validate() error = %v, want ErrInvalidField", err)
	}

	if got := (GreetBody{}).Kind(); got != KindGreet {
		t.Fatalf("GreetBody.Kind() = %q, want %q", got, KindGreet)
	}
	if got := (WhoisBody{}).Kind(); got != KindWhois {
		t.Fatalf("WhoisBody.Kind() = %q, want %q", got, KindWhois)
	}
	if got := (SayBody{}).Kind(); got != KindSay {
		t.Fatalf("SayBody.Kind() = %q, want %q", got, KindSay)
	}
	if got := (CapabilityBody{}).Kind(); got != KindCapability {
		t.Fatalf("CapabilityBody.Kind() = %q, want %q", got, KindCapability)
	}
	if got := (ReceiptBody{}).Kind(); got != KindReceipt {
		t.Fatalf("ReceiptBody.Kind() = %q, want %q", got, KindReceipt)
	}
	if got := (TraceBody{}).Kind(); got != KindTrace {
		t.Fatalf("TraceBody.Kind() = %q, want %q", got, KindTrace)
	}

	directed := Envelope{To: new("reviewer.sess-xyz")}
	if !directed.IsDirected() || directed.IsBroadcast() {
		t.Fatalf("directed envelope helper mismatch")
	}
	broadcast := Envelope{}
	if broadcast.IsDirected() || !broadcast.IsBroadcast() {
		t.Fatalf("broadcast envelope helper mismatch")
	}
}

func TestValidateEnvelopeAndDecodeBodyErrors(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	valid := Envelope{
		Protocol:    ProtocolV0,
		WorkspaceID: testWorkspaceID,
		ID:          "msg_direct_01",
		Kind:        KindSay,
		Channel:     "builders",
		Surface:     new(SurfaceDirect),
		DirectID:    new(testDirectRef().DirectID),
		From:        "coder.sess-abc",
		To:          new("reviewer.sess-xyz"),
		WorkID:      new("work_patch_42"),
		TS:          now.Unix(),
		Body:        mustRawJSON(t, map[string]any{"text": "please review auth.go"}),
	}

	if err := ValidateEnvelope(valid, ValidateOptions{Now: now}); err != nil {
		t.Fatalf("ValidateEnvelope() error = %v", err)
	}

	if _, err := ParseEnvelope([]byte(`{"broken"`), ValidateOptions{Now: now}); !errors.Is(err, ErrInvalidEnvelope) {
		t.Fatalf("ParseEnvelope(invalid json) error = %v, want ErrInvalidEnvelope", err)
	}

	if _, err := DecodeBody(Kind("mystery"), mustRawJSON(t, map[string]any{})); !errors.Is(err, ErrInvalidKind) {
		t.Fatalf("DecodeBody(invalid kind) error = %v, want ErrInvalidKind", err)
	}

	if _, err := DecodeBody(
		KindSay,
		mustRawJSON(t, []string{"not", "an", "object"}),
	); !errors.Is(
		err,
		ErrInvalidField,
	) {
		t.Fatalf("DecodeBody(non-object) error = %v, want ErrInvalidField", err)
	}
}

func TestAdditionalBodyValidationBranches(t *testing.T) {
	t.Parallel()

	if _, err := DecodeBody(KindWhois, mustRawJSON(t, map[string]any{
		"type": "request",
		"peer_card": map[string]any{
			"peer_id":               "reviewer.sess-xyz",
			"profiles_supported":    []string{"agh-network/v0"},
			"capabilities":          []string{"chat.review"},
			"artifacts_supported":   []string{"capability"},
			"trust_modes_supported": []string{"unverified"},
		},
	})); !errors.Is(err, ErrInvalidBody) {
		t.Fatalf("DecodeBody(whois request with peer_card) error = %v, want ErrInvalidBody", err)
	}

	if _, err := DecodeBody(KindGreet, mustRawJSON(t, map[string]any{
		"peer_card": map[string]any{
			"peer_id":               "reviewer.sess-xyz",
			"capabilities":          []string{"chat.review"},
			"artifacts_supported":   []string{"capability"},
			"trust_modes_supported": []string{"unverified"},
		},
	})); !errors.Is(err, ErrInvalidBody) {
		t.Fatalf("DecodeBody(greet missing profiles_supported) error = %v, want ErrInvalidBody", err)
	}

	if _, err := DecodeBody(KindTrace, mustRawJSON(t, map[string]any{
		"state": "unknown",
	})); !errors.Is(err, ErrInvalidField) {
		t.Fatalf("DecodeBody(trace invalid state) error = %v, want ErrInvalidField", err)
	}

	if _, err := DecodeBody(KindReceipt, mustRawJSON(t, map[string]any{
		"for_id": "msg_direct_01",
		"status": "expired",
	})); !errors.Is(err, ErrInvalidBody) {
		t.Fatalf("DecodeBody(receipt missing reason_code) error = %v, want ErrInvalidBody", err)
	}
}

func TestWorkValidationAndTraceMatrix(t *testing.T) {
	t.Parallel()

	valid := Work{
		ID:        "work_patch_42",
		Ref:       testDirectRef(),
		Initiator: "coder.sess-abc",
		Target:    "reviewer.sess-xyz",
		State:     WorkStateWorking,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Work.Validate() error = %v", err)
	}

	invalid := valid
	invalid.Target = "BadPeer"
	if err := invalid.Validate(); !errors.Is(err, ErrInvalidField) {
		t.Fatalf("Work.Validate(invalid target) error = %v, want ErrInvalidField", err)
	}

	openErrEnv := Envelope{
		Protocol:    ProtocolV0,
		WorkspaceID: testWorkspaceID,
		ID:          "msg_trace_01",
		Kind:        KindTrace,
		Channel:     "builders",
		From:        "reviewer.sess-xyz",
		To:          new("coder.sess-abc"),
		WorkID:      new("work_patch_42"),
		TS:          time.Now().Unix(),
		Body:        mustRawJSON(t, map[string]any{"state": "working"}),
	}
	if _, err := OpenWork(withDirectSurface(openErrEnv), time.Time{}); !errors.Is(err, ErrInvalidField) {
		t.Fatalf("OpenWork(non-opener) error = %v, want ErrInvalidField", err)
	}

	matrix := []struct {
		name    string
		current WorkState
		next    WorkState
		want    bool
	}{
		{name: "ShouldAllowSubmittedToWorking", current: WorkStateSubmitted, next: WorkStateWorking, want: true},
		{name: "ShouldRejectSubmittedToSubmitted", current: WorkStateSubmitted, next: WorkStateSubmitted, want: false},
		{name: "ShouldAllowWorkingToCompleted", current: WorkStateWorking, next: WorkStateCompleted, want: true},
		{name: "ShouldRejectWorkingToSubmitted", current: WorkStateWorking, next: WorkStateSubmitted, want: false},
		{name: "ShouldAllowNeedsInputToWorking", current: WorkStateNeedsInput, next: WorkStateWorking, want: true},
		{name: "ShouldAllowNeedsInputToCanceled", current: WorkStateNeedsInput, next: WorkStateCanceled, want: true},
		{name: "ShouldRejectCompletedToWorking", current: WorkStateCompleted, next: WorkStateWorking, want: false},
		{name: "ShouldRejectFailedToWorking", current: WorkStateFailed, next: WorkStateWorking, want: false},
		{name: "ShouldRejectCanceledToWorking", current: WorkStateCanceled, next: WorkStateWorking, want: false},
	}

	for _, tc := range matrix {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := canApplyTrace(tc.current, tc.next); got != tc.want {
				t.Fatalf("canApplyTrace(%q, %q) = %v, want %v", tc.current, tc.next, got, tc.want)
			}
			err := ValidateWorkTransition(tc.current, tc.next)
			if tc.want && err != nil {
				t.Fatalf("ValidateWorkTransition(%q, %q) error = %v", tc.current, tc.next, err)
			}
			if !tc.want && !errors.Is(err, ErrInvalidStateTransition) {
				t.Fatalf(
					"ValidateWorkTransition(%q, %q) error = %v, want ErrInvalidStateTransition",
					tc.current,
					tc.next,
					err,
				)
			}
		})
	}

	t.Run("Should reject all transitions from terminal states", func(t *testing.T) {
		t.Parallel()

		terminalStates := []WorkState{
			WorkStateCompleted,
			WorkStateFailed,
			WorkStateCanceled,
		}
		nextStates := []WorkState{
			WorkStateSubmitted,
			WorkStateWorking,
			WorkStateNeedsInput,
			WorkStateCompleted,
			WorkStateFailed,
			WorkStateCanceled,
		}

		for _, current := range terminalStates {
			for _, next := range nextStates {
				t.Run(fmt.Sprintf("ShouldReject%sTo%s", current, next), func(t *testing.T) {
					t.Parallel()

					if got := canApplyTrace(current, next); got {
						t.Fatalf("canApplyTrace(%q, %q) = %v, want false", current, next, got)
					}
					if err := ValidateWorkTransition(current, next); !errors.Is(err, ErrInvalidStateTransition) {
						t.Fatalf(
							"ValidateWorkTransition(%q, %q) error = %v, want ErrInvalidStateTransition",
							current,
							next,
							err,
						)
					}
				})
			}
		}
	})
}

func TestAdditionalEnvelopeAndLifecycleBranches(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)

	greetMismatch := Envelope{
		Protocol:    ProtocolV0,
		WorkspaceID: testWorkspaceID,
		ID:          "msg_greet_01",
		Kind:        KindGreet,
		Channel:     "builders",
		From:        "coder.sess-abc",
		TS:          now.Unix(),
		Body: mustRawJSON(t, map[string]any{
			"peer_card": map[string]any{
				"peer_id":               "reviewer.sess-xyz",
				"profiles_supported":    []string{"agh-network/v0"},
				"capabilities":          []string{"workspace.patch.apply"},
				"artifacts_supported":   []string{"capability"},
				"trust_modes_supported": []string{"unverified"},
			},
		}),
	}
	if _, err := NormalizeEnvelope(greetMismatch, ValidateOptions{Now: now}); !errors.Is(err, ErrInvalidBody) {
		t.Fatalf("NormalizeEnvelope(greet mismatch) error = %v, want ErrInvalidBody", err)
	}

	receiptMissingWork := Envelope{
		Protocol:    ProtocolV0,
		WorkspaceID: testWorkspaceID,
		ID:          "msg_receipt_01",
		Kind:        KindReceipt,
		Channel:     "builders",
		Surface:     new(SurfaceDirect),
		DirectID:    new(testDirectRef().DirectID),
		From:        "reviewer.sess-xyz",
		TS:          now.Unix(),
		Body: mustRawJSON(t, map[string]any{
			"for_id": "msg_direct_01",
			"status": "accepted",
		}),
	}
	if _, err := NormalizeEnvelope(
		receiptMissingWork,
		ValidateOptions{Now: now},
	); !errors.Is(
		err,
		ErrMissingField,
	) {
		t.Fatalf("NormalizeEnvelope(receipt missing work_id) error = %v, want ErrMissingField", err)
	}

	blankSay, err := DecodeBody(KindSay, mustRawJSON(t, map[string]any{"text": "   "}))
	if !errors.Is(err, ErrInvalidBody) || blankSay != nil {
		t.Fatalf("DecodeBody(blank say) body = %#v, error = %v, want nil + ErrInvalidBody", blankSay, err)
	}

	blankSayNewline, err := DecodeBody(KindSay, mustRawJSON(t, map[string]any{"text": "\n"}))
	if !errors.Is(err, ErrInvalidBody) || blankSayNewline != nil {
		t.Fatalf(
			"DecodeBody(blank say newline) body = %#v, error = %v, want nil + ErrInvalidBody",
			blankSayNewline,
			err,
		)
	}

	if cloned := cloneRawMessage(nil); cloned != nil {
		t.Fatalf("cloneRawMessage(nil) = %#v, want nil", cloned)
	}
	if cloned := cloneRawMessage(jsonBytes(`{"ok":true}`)); string(cloned) != `{"ok":true}` {
		t.Fatalf("cloneRawMessage(non-nil) = %s, want original bytes", string(cloned))
	}

	traceEnv := Envelope{
		Protocol:    ProtocolV0,
		WorkspaceID: testWorkspaceID,
		ID:          "msg_trace_01",
		Kind:        KindTrace,
		Channel:     "builders",
		From:        "reviewer.sess-xyz",
		To:          new("coder.sess-abc"),
		WorkID:      new("work_patch_42"),
		TS:          now.Unix(),
		Body:        mustRawJSON(t, map[string]any{"state": "working"}),
	}
	if _, err := ApplyWorkEnvelope(nil, withDirectSurface(traceEnv), now); !errors.Is(err, ErrWorkNotFound) {
		t.Fatalf("ApplyWorkEnvelope(nil trace) error = %v, want ErrWorkNotFound", err)
	}

	terminal := &Work{
		ID:        "work_patch_42",
		Ref:       testDirectRef(),
		Initiator: "coder.sess-abc",
		Target:    "reviewer.sess-xyz",
		State:     WorkStateCanceled,
		CreatedAt: now,
		UpdatedAt: now,
	}
	receiptEnv := Envelope{
		Protocol:    ProtocolV0,
		WorkspaceID: testWorkspaceID,
		ID:          "msg_receipt_02",
		Kind:        KindReceipt,
		Channel:     "builders",
		From:        "coder.sess-abc",
		To:          new("reviewer.sess-xyz"),
		WorkID:      new("work_patch_42"),
		TS:          now.Unix(),
		Body: mustRawJSON(t, map[string]any{
			"for_id": "msg_direct_01",
			"status": "canceled",
		}),
	}
	got, err := ApplyWorkEnvelope(terminal, withDirectSurface(receiptEnv), now.Add(time.Second))
	if err != nil {
		t.Fatalf("ApplyWorkEnvelope(terminal receipt) error = %v", err)
	}
	if got.Action != LifecycleActionRejectWork {
		t.Fatalf("ApplyWorkEnvelope(terminal receipt).Action = %q, want %q", got.Action, LifecycleActionRejectWork)
	}
	if got.ReasonCode == nil || *got.ReasonCode != ReasonCodeWorkClosed {
		t.Fatalf("ApplyWorkEnvelope(terminal receipt).ReasonCode = %v, want %q", got.ReasonCode, ReasonCodeWorkClosed)
	}
}

func jsonBytes(value string) []byte {
	return []byte(value)
}
