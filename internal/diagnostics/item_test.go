package diagnostics

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	contract "github.com/compozy/agh/internal/diagnosticcontract"
)

func TestNewItemRedactsDiagnosticFields(t *testing.T) {
	t.Parallel()

	t.Run("Should redact message and nested evidence values", func(t *testing.T) {
		t.Parallel()

		item := NewItem(
			"test.provider",
			contract.CodeProviderNotAuthenticated,
			contract.CategoryProvider,
			"Provider token=title-secret",
			"Provider failed with Authorization: Bearer message-secret and claim agh_claim_live_secret_123",
			contract.SeverityWarn,
			contract.FreshnessLive,
			WithSuggestedCommand("agh provider auth login claude"),
			WithEvidence(map[string]any{
				"api_key": "sk-live-secret",
				"stderr":  "token=stderr-secret",
				"nested": map[string]any{
					"access_token": "nested-secret",
					"safe":         "ok",
				},
				"list": []string{"Bearer list-secret", "safe"},
			}),
		)

		raw, err := json.Marshal(item)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		serialized := string(raw)
		for _, leaked := range []string{
			"title-secret",
			"message-secret",
			"agh_claim_live_secret_123",
			"sk-live-secret",
			"stderr-secret",
			"nested-secret",
			"list-secret",
		} {
			if strings.Contains(serialized, leaked) {
				t.Fatalf("DiagnosticItem JSON = %s leaked %q", serialized, leaked)
			}
		}
		if item.Evidence["api_key"] != redactedValue {
			t.Fatalf("Evidence[api_key] = %#v, want redacted marker", item.Evidence["api_key"])
		}
	})
}

func TestNewItemDowngradesInvalidInput(t *testing.T) {
	t.Run("Should downgrade unknown diagnostic code without panicking", func(t *testing.T) {
		logs := captureDiagnosticWarnings(t)

		item := NewItem(
			"",
			"not_registered",
			"not_a_category",
			"",
			"",
			"bad_severity",
			"bad_freshness",
		)

		if item.ID != malformedDiagnosticID {
			t.Fatalf("NewItem().ID = %q, want %q", item.ID, malformedDiagnosticID)
		}
		if item.Code != contract.CodeUnknownComponent {
			t.Fatalf("NewItem().Code = %q, want %q", item.Code, contract.CodeUnknownComponent)
		}
		if item.Category != contract.CategoryDaemon {
			t.Fatalf("NewItem().Category = %q, want %q", item.Category, contract.CategoryDaemon)
		}
		if item.Severity != contract.SeverityCritical {
			t.Fatalf("NewItem().Severity = %q, want %q", item.Severity, contract.SeverityCritical)
		}
		if item.DataFreshness != contract.FreshnessStale {
			t.Fatalf("NewItem().DataFreshness = %q, want %q", item.DataFreshness, contract.FreshnessStale)
		}
		if err := contract.ValidateDiagnosticItem(item); err != nil {
			t.Fatalf("ValidateDiagnosticItem() error = %v", err)
		}
		if got := logs.String(); !strings.Contains(got, "invalid DiagnosticItem downgraded") ||
			!strings.Contains(got, "not_registered") {
			t.Fatalf("diagnostic warning log = %q, want downgrade warning with original code", got)
		}
	})

	t.Run("Should downgrade invalid severity and freshness on known code", func(t *testing.T) {
		logs := captureDiagnosticWarnings(t)

		item := NewItem(
			"test.config_invalid",
			contract.CodeConfigInvalid,
			contract.CategoryConfig,
			"Config invalid",
			"Config could not be applied",
			"bad_severity",
			"bad_freshness",
		)

		if item.Code != contract.CodeConfigInvalid {
			t.Fatalf("NewItem().Code = %q, want %q", item.Code, contract.CodeConfigInvalid)
		}
		if item.Category != contract.CategoryConfig {
			t.Fatalf("NewItem().Category = %q, want %q", item.Category, contract.CategoryConfig)
		}
		if item.Severity != contract.SeverityCritical {
			t.Fatalf("NewItem().Severity = %q, want %q", item.Severity, contract.SeverityCritical)
		}
		if item.DataFreshness != contract.FreshnessStale {
			t.Fatalf("NewItem().DataFreshness = %q, want %q", item.DataFreshness, contract.FreshnessStale)
		}
		if err := contract.ValidateDiagnosticItem(item); err != nil {
			t.Fatalf("ValidateDiagnosticItem() error = %v", err)
		}
		if got := logs.String(); !strings.Contains(got, "bad_severity") ||
			!strings.Contains(got, "bad_freshness") {
			t.Fatalf("diagnostic warning log = %q, want invalid severity and freshness", got)
		}
	})
}

func captureDiagnosticWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()

	var logs bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() {
		slog.SetDefault(original)
	})
	return &logs
}

func TestRedactJSONRedactsRecursiveSecrets(t *testing.T) {
	t.Parallel()

	t.Run("Should redact JSON object values recursively", func(t *testing.T) {
		t.Parallel()

		raw := []byte(
			`{"auth":{"access_token":"json-secret"},"items":[{"stderr":"token=item-secret"}],"safe":"ok"}`,
		)
		redacted, err := RedactJSON(raw)
		if err != nil {
			t.Fatalf("RedactJSON() error = %v", err)
		}
		got := string(redacted)
		if strings.Contains(got, "json-secret") || strings.Contains(got, "item-secret") {
			t.Fatalf("RedactJSON() = %s, want secrets redacted", got)
		}
		if !strings.Contains(got, `"safe":"ok"`) {
			t.Fatalf("RedactJSON() = %s, want safe field preserved", got)
		}
	})

	t.Run("Should preserve non-string JSON scalar types", func(t *testing.T) {
		t.Parallel()

		redacted, err := RedactJSON([]byte(`{"count":7,"ratio":0.5,"enabled":true,"missing":null}`))
		if err != nil {
			t.Fatalf("RedactJSON() error = %v", err)
		}
		var got map[string]any
		if err := json.Unmarshal(redacted, &got); err != nil {
			t.Fatalf("json.Unmarshal(RedactJSON()) error = %v", err)
		}
		if got["count"] != float64(7) || got["ratio"] != 0.5 || got["enabled"] != true || got["missing"] != nil {
			t.Fatalf("RedactJSON() scalars = %#v, want numeric/bool/null types preserved", got)
		}
	})
}

func TestStructuredErrorCarriesDiagnosticAndCause(t *testing.T) {
	t.Parallel()

	t.Run("Should render command and preserve cause", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("connect: token=cause-secret")
		item := NewItem(
			"cli.daemon_unavailable",
			contract.CodeDaemonUnavailable,
			contract.CategoryDaemon,
			"Daemon unavailable",
			"socket token=message-secret",
			contract.SeverityError,
			contract.FreshnessOffline,
			WithSuggestedCommand("agh daemon start"),
		)
		err := NewStructuredError(item, cause)
		if !errors.Is(err, cause) {
			t.Fatalf("errors.Is() = false, want cause preserved")
		}
		if !strings.Contains(err.Error(), "agh daemon start") {
			t.Fatalf("Error() = %q, want suggested command", err.Error())
		}
		if strings.Contains(err.Error(), "message-secret") {
			t.Fatalf("Error() = %q leaked message secret", err.Error())
		}
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			t.Fatal("errors.Unwrap() = nil, want redacted cause")
		}
		if strings.Contains(unwrapped.Error(), "cause-secret") {
			t.Fatalf("Unwrap().Error() = %q leaked cause secret", unwrapped.Error())
		}
		extracted, ok := ItemFromError(err)
		if !ok {
			t.Fatal("ItemFromError() ok = false, want true")
		}
		if extracted.Code != contract.CodeDaemonUnavailable {
			t.Fatalf("ItemFromError().Code = %q, want %q", extracted.Code, contract.CodeDaemonUnavailable)
		}
	})
}
