package tools

import (
	"encoding/json"
	"testing"
)

func TestCanonicalJSONShouldProduceStableJCSBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  json.RawMessage
		want string
	}{
		{
			name: "Should Sort Object Keys",
			raw:  json.RawMessage(`{"b":true,"a":{"z":null,"m":"<tag>"}}`),
			want: `{"a":{"m":"<tag>","z":null},"b":true}`,
		},
		{
			name: "Should Preserve Array Order",
			raw:  json.RawMessage(`[{"z":2,"a":1},"text",false]`),
			want: `[{"a":1,"z":2},"text",false]`,
		},
		{
			name: "Should CanonicalizeZero",
			raw:  json.RawMessage(`-0`),
			want: `0`,
		},
		{
			name: "Should CanonicalizeExponent",
			raw:  json.RawMessage(`1.2e+3`),
			want: `1200`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := CanonicalJSON(tt.raw)
			if err != nil {
				t.Fatalf("CanonicalJSON() error = %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("CanonicalJSON() = %q, want %q", string(got), tt.want)
			}
		})
	}
}

func TestCanonicalJSONShouldRejectInvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  json.RawMessage
	}{
		{name: "Should Reject Empty Input"},
		{name: "Should Reject Multiple JSON Values", raw: json.RawMessage(`{"a":1} {"b":2}`)},
		{name: "Should Reject Malformed JSON", raw: json.RawMessage(`{`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := CanonicalJSON(tt.raw)
			requireReason(t, err, ReasonSchemaInvalid)
		})
	}
}

func TestSchemaDigestShouldHashCanonicalSchemaObject(t *testing.T) {
	t.Parallel()

	t.Run("Should Hash Canonical Schema Object", func(t *testing.T) {
		t.Parallel()

		schema := json.RawMessage(`{"required":["query"],"properties":{"query":{"type":"string"}},"type":"object"}`)
		got, err := SchemaDigest(schema)
		if err != nil {
			t.Fatalf("SchemaDigest() error = %v", err)
		}
		const want = "1dc63095e8672403bbe40fa26719d175e695c0167f6daad6b9655f6506491f01"
		if got != want {
			t.Fatalf("SchemaDigest() = %q, want %q", got, want)
		}
	})
}

func TestSchemaDigestShouldRejectNonObjectSchema(t *testing.T) {
	t.Parallel()

	t.Run("Should Reject Non Object Schema", func(t *testing.T) {
		t.Parallel()

		_, err := SchemaDigest(json.RawMessage(`["not","object"]`))
		requireReason(t, err, ReasonSchemaInvalid)
	})
}

func TestExtensionToolRuntimeDescriptorValidateShouldRequireReconciliationFields(t *testing.T) {
	t.Parallel()

	valid := ExtensionToolRuntimeDescriptor{
		ID:                "ext__linear__lookup",
		Handler:           "lookup",
		InputSchemaDigest: "1dc63095e8672403bbe40fa26719d175e695c0167f6daad6b9655f6506491f01",
		ReadOnly:          true,
		Risk:              RiskRead,
		Capabilities:      []string{"tool.provider"},
	}
	t.Run("Should Accept Matching Runtime Descriptor", func(t *testing.T) {
		t.Parallel()

		if err := valid.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
	})
	t.Run("Should Reject Invalid ID", func(t *testing.T) {
		t.Parallel()

		invalidID := valid
		invalidID.ID = "Bad"
		requireReason(t, invalidID.Validate(), ReasonIDInvalidFormat)
	})
	t.Run("Should Reject Missing Handler", func(t *testing.T) {
		t.Parallel()

		missingHandler := valid
		missingHandler.Handler = ""
		requireReason(t, missingHandler.Validate(), ReasonHandlerMissing)
	})
	t.Run("Should Reject Missing Input Digest", func(t *testing.T) {
		t.Parallel()

		missingDigest := valid
		missingDigest.InputSchemaDigest = ""
		requireReason(t, missingDigest.Validate(), ReasonRuntimeDescriptorMismatch)
	})
	t.Run("Should Reject Invalid Risk", func(t *testing.T) {
		t.Parallel()

		invalidRisk := valid
		invalidRisk.Risk = RiskClass("danger")
		requireReason(t, invalidRisk.Validate(), ReasonPolicyDenied)
	})
}
