package cli

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestParseRequiredJSONRawMessage(t *testing.T) {
	t.Parallel()

	t.Run("Should trim and preserve valid JSON", func(t *testing.T) {
		t.Parallel()

		payload, err := parseRequiredJSONRawMessage("  {\"schema\":\"compozy.test\"}  ")
		if err != nil {
			t.Fatalf("parseRequiredJSONRawMessage() error = %v", err)
		}
		if got, want := string(payload), `{"schema":"compozy.test"}`; got != want {
			t.Fatalf("parseRequiredJSONRawMessage() = %q, want %q", got, want)
		}
	})

	t.Run("Should reject empty JSON", func(t *testing.T) {
		t.Parallel()

		_, err := parseRequiredJSONRawMessage(" \t\n ")
		if !errors.Is(err, errEmptyJSONFlag) {
			t.Fatalf("parseRequiredJSONRawMessage() error = %v, want %v", err, errEmptyJSONFlag)
		}
	})

	t.Run("Should reject invalid JSON", func(t *testing.T) {
		t.Parallel()

		_, err := parseRequiredJSONRawMessage("{")
		if err == nil {
			t.Fatal("parseRequiredJSONRawMessage() error = nil, want non-nil")
		}
		if _, ok := errors.AsType[*json.SyntaxError](err); !ok {
			t.Fatalf("parseRequiredJSONRawMessage() error = %v, want *json.SyntaxError", err)
		}
	})
}
