package contracts

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	redactpkg "github.com/compozy/compozy/internal/redact"
)

func TestContractPreservingRedaction(t *testing.T) {
	t.Parallel()

	t.Run("Should hash-redact a claim token in free text and stay valid", func(t *testing.T) {
		t.Parallel()

		contract := mustContract(t, json.RawMessage(`{
			"type":"object","properties":{"note":{"type":"string"}},"required":["note"]
		}`))
		payload := json.RawMessage(`{"note":"token COMPOZY_CLAIM_super-secret-value"}`)
		redacted, records, err := RedactPreservingContract(contract, payload)
		if err != nil {
			t.Fatalf("RedactPreservingContract() error = %v", err)
		}
		if len(records) != 1 || strings.Contains(string(redacted), "super-secret-value") ||
			!strings.Contains(string(redacted), "sha256:") {
			t.Fatalf("redacted = %s, records = %#v; want hashed token", redacted, records)
		}
	})

	t.Run("Should reject redaction that violates an enum-bound field", func(t *testing.T) {
		t.Parallel()

		contract := mustContract(t, json.RawMessage(`{
			"type":"object",
			"properties":{"note":{"type":"string","enum":["COMPOZY_CLAIM_super-secret-value"]}},
			"required":["note"]
		}`))
		_, _, err := RedactPreservingContract(
			contract,
			json.RawMessage(`{"note":"COMPOZY_CLAIM_super-secret-value"}`),
		)
		if !IsCode(err, CodeRedactionConflict) {
			t.Fatalf("RedactPreservingContract() error = %v, want %s", err, CodeRedactionConflict)
		}
	})

	t.Run("Should reject denylisted keys and preserve hash fields", func(t *testing.T) {
		t.Parallel()

		contract := mustContract(t, json.RawMessage(`{"type":"object"}`))
		_, _, err := RedactPreservingContract(
			contract,
			json.RawMessage(`{"apikey":"plain-value","access_token":"another-value","token_hash":"safe-hash"}`),
		)
		if !IsCode(err, CodeRedactionConflict) || !strings.Contains(err.Error(), "$.apikey") {
			t.Fatalf("RedactPreservingContract() error = %v, want secret-field rejection", err)
		}
	})

	t.Run("Should bound and label split-secret best-effort rendering", func(t *testing.T) {
		t.Parallel()

		rendered := RenderUntrusted("call-result", map[string]string{
			"part_a": "COMPOZY_",
			"part_b": "CLAIM_split-across-fields",
		}, 64)
		if len(rendered) > 64 {
			t.Fatalf("RenderUntrusted() bytes = %d, want <= 64", len(rendered))
		}
		if !strings.HasPrefix(rendered, "[untrusted call-result]") {
			t.Fatalf("RenderUntrusted() = %q, want label", rendered)
		}
		if !strings.HasSuffix(rendered, "[/untrusted]") {
			t.Fatalf("RenderUntrusted() = %q, want closing label", rendered)
		}
	})

	t.Run("Should reject required secret-shaped contract fields at authoring time", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry(newMemoryRegistryStore())
		_, err := registry.Pin(context.Background(), json.RawMessage(`{
			"type":"object","properties":{"api_key":{"type":"string"}},"required":["api_key"]
		}`))
		if !IsCode(err, CodeExpectInvalid) || !strings.Contains(err.Error(), "*_hash") {
			t.Fatalf("Pin(secret field) error = %v, want authoring guidance", err)
		}
	})

	t.Run("Should reject secret-shaped fields reached through reusable schemas", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry(newMemoryRegistryStore())
		for _, schema := range []json.RawMessage{
			json.RawMessage(`{
				"type":"object","properties":{"nested":{"$ref":"#/$defs/secret"}},
				"$defs":{"secret":{"type":"object","properties":{"claim_token":{"type":"string"}},"required":["claim_token"]}}
			}`),
			json.RawMessage(`{
				"type":"object","dependentSchemas":{"mode":{"type":"object","properties":{"password":{"type":"string"}},"required":["password"]}}
			}`),
		} {
			_, err := registry.Pin(context.Background(), schema)
			if !IsCode(err, CodeExpectInvalid) || !strings.Contains(err.Error(), "*_hash") {
				t.Fatalf("Pin(reusable secret field) error = %v, want authoring guidance", err)
			}
		}
	})
}

func TestSanitizeText(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve clean text", func(t *testing.T) {
		t.Parallel()

		clean, redactions, reject := SanitizeText("ordinary mailbox body")
		if clean != "ordinary mailbox body" || len(redactions) != 0 || reject {
			t.Fatalf("SanitizeText(clean) = %q, %#v, %v", clean, redactions, reject)
		}
	})

	t.Run("Should hash-redact secret material inside prose", func(t *testing.T) {
		t.Parallel()

		clean, redactions, reject := SanitizeText("use COMPOZY_CLAIM_secret-value now")
		if reject || len(redactions) != 1 || strings.Contains(clean, "secret-value") ||
			!strings.Contains(clean, "sha256:") {
			t.Fatalf("SanitizeText(secret) = %q, %#v, %v", clean, redactions, reject)
		}
	})

	t.Run("Should reject structurally all-secret text", func(t *testing.T) {
		t.Parallel()

		clean, redactions, reject := SanitizeText("COMPOZY_CLAIM_secret-value")
		if clean != "" || len(redactions) != 1 || !reject {
			t.Fatalf("SanitizeText(all secret) = %q, %#v, %v", clean, redactions, reject)
		}
	})

	t.Run("Should classify caller-registered secret material", func(t *testing.T) {
		t.Parallel()

		secret := "runtime-required-secret-value"
		cleanup := redactpkg.RegisterRequiredSecret(secret)
		t.Cleanup(cleanup)
		clean, redactions, reject := SanitizeText("prefix " + secret + " suffix")
		if reject || len(redactions) == 0 || strings.Contains(clean, secret) {
			t.Fatalf("SanitizeText(dynamic secret) = %q, %#v, %v", clean, redactions, reject)
		}
	})
}

type fakeEntityCatalog struct {
	known map[EntityKind]map[string]bool
}

func (f fakeEntityCatalog) EntityExists(_ context.Context, kind EntityKind, value string) (bool, error) {
	return f.known[kind][value], nil
}

func TestEntityAnnotationWalk(t *testing.T) {
	t.Parallel()

	contract := mustContract(t, json.RawMessage(`{
		"type":"object",
		"properties":{"reviewer":{"type":"string","x-compozy-kind":"agent"}},
		"required":["reviewer"]
	}`))
	catalog := fakeEntityCatalog{known: map[EntityKind]map[string]bool{
		EntityAgent: {"reviewer": true},
	}}

	t.Run("Should resolve a known annotated agent name", func(t *testing.T) {
		t.Parallel()

		issues, err := ValidateEntities(
			context.Background(),
			contract,
			json.RawMessage(`{"reviewer":"reviewer"}`),
			catalog,
		)
		if err != nil || len(issues) != 0 {
			t.Fatalf("ValidateEntities(known) = %#v, %v; want no issues", issues, err)
		}
	})

	t.Run("Should return a typed-path issue for an unknown entity", func(t *testing.T) {
		t.Parallel()

		issues, err := ValidateEntities(
			context.Background(),
			contract,
			json.RawMessage(`{"reviewer":"missing"}`),
			catalog,
		)
		if err != nil {
			t.Fatalf("ValidateEntities(unknown) error = %v", err)
		}
		if len(issues) != 1 || issues[0].Path != "$.reviewer" ||
			!strings.Contains(issues[0].Message, "does not exist") {
			t.Fatalf("ValidateEntities(unknown) issues = %#v", issues)
		}
	})
}

func mustContract(t *testing.T, schema json.RawMessage) Contract {
	t.Helper()
	canonical, err := normalizeSchema(schema)
	if err != nil {
		t.Fatalf("normalizeSchema() error = %v", err)
	}
	return Contract{Digest: "sha256:test", Schema: canonical}
}
