package contract

import (
	"strings"
	"testing"
)

func TestBridgeCheckRecordHelpersDoNotCarryProviderErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should build a passing record without diagnostic prose", func(t *testing.T) {
		t.Parallel()

		got := PassCheck("provider.identity")
		if got.Status != BridgeCheckStatusPass || got.Remediation != "" {
			t.Fatalf("PassCheck() = %#v, want pass with empty remediation", got)
		}
	})

	t.Run("Should build an actionable skipped record", func(t *testing.T) {
		t.Parallel()

		got := SkippedCheck("webhook.reachability", "Enable the bridge and retry.")
		if got.Status != BridgeCheckStatusSkipped || got.Remediation == "" {
			t.Fatalf("SkippedCheck() = %#v, want actionable skipped record", got)
		}
	})

	t.Run("Should identify a missing binding without accepting a raw error", func(t *testing.T) {
		t.Parallel()

		got := FailedSecretCheck("provider.identity", "bot_token")
		if got.Status != BridgeCheckStatusFail || !strings.Contains(got.Remediation, "bot_token") {
			t.Fatalf("FailedSecretCheck() = %#v, want binding remediation", got)
		}
	})
}

func TestBridgeControlWireValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should validate closed methods and statuses", func(t *testing.T) {
		t.Parallel()

		if got, want := strings.Join(
			ControlMethodValues(),
			",",
		), "bridges/check,bridges/webhook/register"; got != want {
			t.Fatalf("ControlMethodValues() = %q, want %q", got, want)
		}
		validMethods := []struct {
			name  string
			value ControlMethod
		}{
			{name: "Should accept the check method", value: ControlMethodCheck},
			{name: "Should accept the webhook registration method", value: ControlMethodWebhookRegister},
		}
		for _, test := range validMethods {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				if err := test.value.Validate(); err != nil {
					t.Fatalf("ControlMethod(%q).Validate() error = %v", test.value, err)
				}
			})
		}
		invalidMethods := []struct {
			name  string
			value ControlMethod
		}{
			{name: "Should reject an empty method", value: ""},
			{name: "Should reject a non-canonical method", value: " bridges/check "},
			{name: "Should reject an unknown method", value: "unknown"},
		}
		for _, test := range invalidMethods {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				if err := test.value.Validate(); err == nil {
					t.Fatalf("ControlMethod(%q).Validate() error = nil", test.value)
				}
			})
		}

		if got, want := strings.Join(BridgeCheckStatusValues(), ","), "pass,warn,fail,skipped"; got != want {
			t.Fatalf("BridgeCheckStatusValues() = %q, want %q", got, want)
		}
		validStatuses := []struct {
			name  string
			value BridgeCheckStatus
		}{
			{name: "Should accept pass status", value: BridgeCheckStatusPass},
			{name: "Should accept warn status", value: BridgeCheckStatusWarn},
			{name: "Should accept fail status", value: BridgeCheckStatusFail},
			{name: "Should accept skipped status", value: BridgeCheckStatusSkipped},
		}
		for _, test := range validStatuses {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				if err := test.value.Validate(); err != nil {
					t.Fatalf("BridgeCheckStatus(%q).Validate() error = %v", test.value, err)
				}
			})
		}
		invalidStatuses := []struct {
			name  string
			value BridgeCheckStatus
		}{
			{name: "Should reject an empty status", value: ""},
			{name: "Should reject a non-canonical status", value: " pass "},
			{name: "Should reject an unknown status", value: "unknown"},
		}
		for _, test := range invalidStatuses {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				if err := test.value.Validate(); err == nil {
					t.Fatalf("BridgeCheckStatus(%q).Validate() error = nil", test.value)
				}
			})
		}
	})

	t.Run("Should validate actionable redacted check records", func(t *testing.T) {
		t.Parallel()

		valid := []struct {
			name   string
			record BridgeCheckRecord
		}{
			{
				name:   "Should accept a passing record",
				record: BridgeCheckRecord{Check: "provider.identity", Status: BridgeCheckStatusPass},
			},
			{
				name: "Should accept an actionable warning",
				record: BridgeCheckRecord{
					Check:       "provider.scope",
					Status:      BridgeCheckStatusWarn,
					Remediation: "Grant the required scope.",
				},
			},
			{
				name: "Should accept an actionable skipped record",
				record: BridgeCheckRecord{
					Check:       "webhook",
					Status:      BridgeCheckStatusSkipped,
					Remediation: "Enable the bridge.",
				},
			},
		}
		for _, test := range valid {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				if err := test.record.Validate(); err != nil {
					t.Fatalf("BridgeCheckRecord(%#v).Validate() error = %v", test.record, err)
				}
			})
		}
		invalid := []struct {
			name   string
			record BridgeCheckRecord
		}{
			{name: "Should reject an empty record", record: BridgeCheckRecord{}},
			{
				name:   "Should reject an unknown record status",
				record: BridgeCheckRecord{Check: "provider.identity", Status: "unknown", Remediation: "Review config."},
			},
			{
				name:   "Should reject a failure without remediation",
				record: BridgeCheckRecord{Check: "provider.identity", Status: BridgeCheckStatusFail},
			},
			{
				name:   "Should reject a sensitive check name",
				record: BridgeCheckRecord{Check: "api_key=secret", Status: BridgeCheckStatusPass},
			},
			{
				name: "Should reject a sensitive remediation",
				record: BridgeCheckRecord{
					Check:       "provider.identity",
					Status:      BridgeCheckStatusFail,
					Remediation: "Replace xoxb-secret-token.",
				},
			},
		}
		for _, test := range invalid {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				if err := test.record.Validate(); err == nil {
					t.Fatalf("BridgeCheckRecord(%#v).Validate() error = nil", test.record)
				}
			})
		}

		validRecords := []BridgeCheckRecord{valid[0].record, valid[1].record, valid[2].record}
		if err := (BridgeCheckResponse{Checks: validRecords}).Validate(); err != nil {
			t.Fatalf("BridgeCheckResponse.Validate() error = %v", err)
		}
		if err := (BridgeCheckResponse{}).Validate(); err == nil {
			t.Fatal("BridgeCheckResponse(empty).Validate() error = nil")
		}
		if err := (BridgeCheckResponse{Checks: []BridgeCheckRecord{valid[0].record, valid[0].record}}).Validate(); err == nil {
			t.Fatal("BridgeCheckResponse(duplicate).Validate() error = nil")
		}
		if err := (BridgeCheckResponse{Checks: []BridgeCheckRecord{{Check: "x", Status: "unknown"}}}).Validate(); err == nil {
			t.Fatal("BridgeCheckResponse(invalid check).Validate() error = nil")
		}
	})

	t.Run("Should validate request and webhook response shapes", func(t *testing.T) {
		t.Parallel()

		if err := (BridgeCheckRequest{BridgeInstanceID: "brg-1"}).Validate(); err != nil {
			t.Fatalf("BridgeCheckRequest.Validate() error = %v", err)
		}
		if err := (BridgeCheckRequest{}).Validate(); err == nil {
			t.Fatal("BridgeCheckRequest(empty).Validate() error = nil")
		}
		if err := (BridgeWebhookRegistrationRequest{BridgeInstanceID: "brg-1"}).Validate(); err != nil {
			t.Fatalf("BridgeWebhookRegistrationRequest.Validate() error = %v", err)
		}
		if err := (BridgeWebhookRegistrationRequest{}).Validate(); err == nil {
			t.Fatal("BridgeWebhookRegistrationRequest(empty).Validate() error = nil")
		}
		if err := (BridgeWebhookRegistrationResponse{Status: BridgeCheckStatusPass}).Validate(); err != nil {
			t.Fatalf("BridgeWebhookRegistrationResponse(pass).Validate() error = %v", err)
		}
		if err := (BridgeWebhookRegistrationResponse{Status: BridgeCheckStatusSkipped}).Validate(); err == nil {
			t.Fatal("BridgeWebhookRegistrationResponse(skipped without remediation).Validate() error = nil")
		}
	})
}
