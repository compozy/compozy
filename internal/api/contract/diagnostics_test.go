package contract_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
)

func TestDiagnosticItemContract(t *testing.T) {
	t.Parallel()

	t.Run("Should marshal and round trip every field", func(t *testing.T) {
		t.Parallel()

		item := contract.DiagnosticItem{
			ID:               "doctor.provider.cli",
			Code:             contract.CodeProviderCLIMissing,
			Severity:         contract.SeverityError,
			Category:         contract.CategoryProvider,
			Title:            "Provider CLI missing",
			Message:          "Install the provider CLI.",
			SuggestedCommand: "brew install provider",
			DocURL:           "https://docs.agh.network/runtime/provider-auth",
			DataFreshness:    contract.FreshnessLive,
			Evidence: map[string]any{
				"provider": "claude",
			},
		}

		raw, err := json.Marshal(item)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		var decoded contract.DiagnosticItem
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if decoded.ID != item.ID ||
			decoded.Code != item.Code ||
			decoded.Severity != item.Severity ||
			decoded.Category != item.Category ||
			decoded.SuggestedCommand != item.SuggestedCommand ||
			decoded.DataFreshness != item.DataFreshness {
			t.Fatalf("DiagnosticItem round trip = %#v, want %#v", decoded, item)
		}
	})

	t.Run("Should validate registry without duplicate or unknown codes", func(t *testing.T) {
		t.Parallel()

		if err := contract.ValidateDiagnosticRegistry(); err != nil {
			t.Fatalf("ValidateDiagnosticRegistry() error = %v", err)
		}
		if !contract.IsDiagnosticCode(contract.CodeConfigInvalid) {
			t.Fatalf("IsDiagnosticCode(%q) = false, want true", contract.CodeConfigInvalid)
		}
		if contract.IsDiagnosticCode("not_registered") {
			t.Fatal("IsDiagnosticCode(not_registered) = true, want false")
		}
		got, ok := contract.DiagnosticCodeCategory(contract.CodeConfigInvalid)
		if !ok || got != contract.CategoryConfig {
			t.Fatalf("DiagnosticCodeCategory(%q) = %q, %v; want %q, true",
				contract.CodeConfigInvalid,
				got,
				ok,
				contract.CategoryConfig,
			)
		}
	})

	t.Run("Should reject unknown code in payload validation", func(t *testing.T) {
		t.Parallel()

		err := contract.ValidateDiagnosticItem(contract.DiagnosticItem{
			ID:            "doctor.invalid",
			Code:          "not_registered",
			Severity:      contract.SeverityError,
			Category:      contract.CategoryDaemon,
			Title:         "Invalid",
			Message:       "Invalid",
			DataFreshness: contract.FreshnessLive,
		})
		if err == nil {
			t.Fatal("ValidateDiagnosticItem() error = nil, want unknown code failure")
		}
		if !strings.Contains(err.Error(), "not_registered") {
			t.Fatalf("ValidateDiagnosticItem() error = %v, want unknown code detail", err)
		}
	})

	t.Run("Should reject code category mismatches", func(t *testing.T) {
		t.Parallel()

		err := contract.ValidateDiagnosticItem(contract.DiagnosticItem{
			ID:            "doctor.invalid_category",
			Code:          contract.CodeConfigInvalid,
			Severity:      contract.SeverityError,
			Category:      contract.CategoryDaemon,
			Title:         "Invalid",
			Message:       "Invalid",
			DataFreshness: contract.FreshnessLive,
		})
		if err == nil {
			t.Fatal("ValidateDiagnosticItem() error = nil, want category mismatch failure")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "category") {
			t.Fatalf("ValidateDiagnosticItem() error = %v, want category mismatch detail", err)
		}
	})
}
