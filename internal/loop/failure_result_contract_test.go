package loop

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func TestInspectPayloadFailureShouldApplyDeclaredFailureContract(t *testing.T) {
	t.Parallel()

	t.Run("Should classify the built in error field as payload declared", func(t *testing.T) {
		t.Parallel()

		inspection := InspectPayloadFailure(json.RawMessage(`{"error":"boom"}`), nil)
		assertPayloadFailure(t, inspection, "boom")
		classified := ClassifyNodeFailure(FailureEvidence{PayloadFailure: inspection.Failure})
		if classified.Class != FailurePayloadDeclared || classified.RetryEligible {
			t.Fatalf("ClassifyNodeFailure() = %#v, want non-retryable payload_declared", classified)
		}
	})

	t.Run("Should classify false success indicators", func(t *testing.T) {
		t.Parallel()

		for _, raw := range []json.RawMessage{json.RawMessage(`{"success":false}`), json.RawMessage(`{"success":"False"}`)} {
			inspection := InspectPayloadFailure(raw, nil)
			assertPayloadFailure(t, inspection, "payload field success declared failure")
		}
	})

	t.Run("Should use a custom failure and message field without built ins", func(t *testing.T) {
		t.Parallel()

		contract := &dsl.ResultContract{FailureField: "err", MessageField: "err.message"}
		inspection := InspectPayloadFailure(
			json.RawMessage(`{"error":"built-in","err":{"message":"custom"}}`),
			contract,
		)
		assertPayloadFailure(t, inspection, "custom")
		if inspection.Failure.Cause == "built-in" {
			t.Fatal("InspectPayloadFailure() evaluated built-ins despite a custom contract")
		}
	})

	t.Run("Should let empty scalar errors pass and fail empty containers", func(t *testing.T) {
		t.Parallel()

		for _, raw := range []json.RawMessage{json.RawMessage(`{"error":""}`), json.RawMessage(`{"error":null}`)} {
			inspection := InspectPayloadFailure(raw, nil)
			if inspection.Failure != nil || inspection.Diagnostic != nil {
				t.Fatalf("InspectPayloadFailure(%s) = %#v, want no failure", raw, inspection)
			}
		}
		for _, raw := range []json.RawMessage{json.RawMessage(`{"error":{}}`), json.RawMessage(`{"error":[]}`)} {
			inspection := InspectPayloadFailure(raw, nil)
			assertPayloadFailure(t, inspection, "declared error was empty")
		}
	})

	t.Run("Should let error win over a success indicator", func(t *testing.T) {
		t.Parallel()

		inspection := InspectPayloadFailure(json.RawMessage(`{"error":"winner","success":true}`), nil)
		assertPayloadFailure(t, inspection, "winner")
	})

	t.Run("Should skip binary and oversized payload inspection", func(t *testing.T) {
		t.Parallel()

		payloads := []json.RawMessage{
			{0xff, 0xfe},
			bytes.Repeat([]byte{'x'}, payloadInspectionMaxBytes+1),
		}
		for _, payload := range payloads {
			inspection := InspectPayloadFailure(payload, nil)
			if inspection.Failure != nil || inspection.Diagnostic == nil {
				t.Fatalf("InspectPayloadFailure() = %#v, want skipped diagnostic", inspection)
			}
			if inspection.Diagnostic.Code != payloadInspectionSkipped {
				t.Fatalf("diagnostic code = %q, want %q", inspection.Diagnostic.Code, payloadInspectionSkipped)
			}
		}
	})

	t.Run("Should sanitize and bound a declared failure once", func(t *testing.T) {
		t.Parallel()

		secret := "compozy_claim_failure_secret"
		inspection := InspectPayloadFailure(
			json.RawMessage(`{"error":"`+secret+` `+strings.Repeat("x", actionFailureTextLimit*2)+`"}`),
			nil,
		)
		if inspection.Failure == nil {
			t.Fatal("InspectPayloadFailure() failure is nil")
		}
		if strings.Contains(inspection.Failure.Cause, secret) {
			t.Fatalf("failure cause leaked secret: %q", inspection.Failure.Cause)
		}
		if len(inspection.Failure.Cause) > actionFailureTextLimit {
			t.Fatalf("failure cause bytes = %d, want <= %d", len(inspection.Failure.Cause), actionFailureTextLimit)
		}
		if !strings.HasSuffix(inspection.Failure.Cause, "...[truncated]") {
			t.Fatalf("failure cause = %q, want truncation marker", inspection.Failure.Cause)
		}
	})
}

func assertPayloadFailure(t *testing.T, inspection PayloadInspection, wantCause string) {
	t.Helper()
	if inspection.Diagnostic != nil {
		t.Fatalf("InspectPayloadFailure() diagnostic = %#v, want none", inspection.Diagnostic)
	}
	if inspection.Failure == nil {
		t.Fatal("InspectPayloadFailure() failure is nil")
	}
	if inspection.Failure.Code != payloadDeclaredCode || inspection.Failure.Cause != wantCause {
		t.Fatalf("InspectPayloadFailure() = %#v, want code %q cause %q", inspection, payloadDeclaredCode, wantCause)
	}
}
