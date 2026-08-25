package contracts

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestResultBudgets(t *testing.T) {
	t.Parallel()

	config := CallsResultsConfig{
		DefaultBudget: ByteBudget{MaxBytes: 256 << 10, Overflow: OverflowStore},
		MaxBudget:     4 << 20,
	}

	t.Run("Should retain a whole over-budget payload with a bounded preview", func(t *testing.T) {
		t.Parallel()

		payload := json.RawMessage(bytes.Repeat([]byte("x"), 300<<10))
		outcome, err := EnforceBudget(config.DefaultBudget, payload)
		if err != nil {
			t.Fatalf("EnforceBudget() error = %v", err)
		}
		if !outcome.Overflowed || len(outcome.Payload) != len(payload) {
			t.Fatalf("EnforceBudget() outcome = %#v, want whole overflow", outcome)
		}
		if len(outcome.Preview) > config.DefaultBudget.MaxBytes {
			t.Fatalf("Preview bytes = %d, want <= %d", len(outcome.Preview), config.DefaultBudget.MaxBytes)
		}
	})

	t.Run("Should reject an over-budget payload under reject policy", func(t *testing.T) {
		t.Parallel()

		_, err := EnforceBudget(ByteBudget{MaxBytes: 3, Overflow: OverflowReject}, json.RawMessage("1234"))
		if !IsCode(err, CodeResultOverBudget) {
			t.Fatalf("EnforceBudget() error = %v, want %s", err, CodeResultOverBudget)
		}
	})

	t.Run("Should reject an override above the configured maximum", func(t *testing.T) {
		t.Parallel()

		override := ByteBudget{MaxBytes: config.MaxBudget + 1, Overflow: OverflowStore}
		_, err := ResolveBudget(&override, config)
		if err == nil {
			t.Fatal("ResolveBudget() error = nil, want cap rejection")
		}
	})

	t.Run("Should accept the exact byte boundary and apply overflow at plus one", func(t *testing.T) {
		t.Parallel()

		budget := ByteBudget{MaxBytes: 4, Overflow: OverflowStore}
		exact, err := EnforceBudget(budget, json.RawMessage("1234"))
		if err != nil || exact.Overflowed {
			t.Fatalf("EnforceBudget(exact) = %#v, %v; want accepted", exact, err)
		}
		plusOne, err := EnforceBudget(budget, json.RawMessage("12345"))
		if err != nil || !plusOne.Overflowed {
			t.Fatalf("EnforceBudget(+1) = %#v, %v; want overflow", plusOne, err)
		}
	})
}

func TestCandidateExtraction(t *testing.T) {
	t.Parallel()

	t.Run("Should find fenced prose-wrapped and balanced objects", func(t *testing.T) {
		t.Parallel()

		for _, input := range []string{
			"\x60\x60\x60json\n{\"answer\":1}\n\x60\x60\x60",
			"before {\"answer\":2} after",
			"before {\"answer\":{\"nested\":true}} after",
		} {
			if candidate, ok := ExtractCandidate(input); !ok || !json.Valid(candidate) {
				t.Fatalf("ExtractCandidate(%q) = %s, %v; want JSON object", input, candidate, ok)
			}
		}
	})

	t.Run("Should return candidates newest-first for newest-valid selection", func(t *testing.T) {
		t.Parallel()

		candidates := ExtractCandidates("first {\"valid\":true} then {\"invalid\":true}")
		if got, want := string(candidates[0]), "{\"invalid\":true}"; got != want {
			t.Fatalf("first candidate = %s, want newest %s", got, want)
		}
		candidates = ExtractCandidates("first {\"valid\":false} then {\"valid\":true}")
		if got, want := string(candidates[0]), "{\"valid\":true}"; got != want {
			t.Fatalf("first candidate = %s, want newest valid %s", got, want)
		}
	})
}
