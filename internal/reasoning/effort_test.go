package reasoning

import (
	"slices"
	"testing"
)

func TestEffortVocabulary(t *testing.T) {
	t.Parallel()

	for _, effort := range []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"} {
		t.Run("Should accept "+effort, func(t *testing.T) {
			t.Parallel()

			if !IsValid(effort) {
				t.Fatalf("IsValid(%q) = false, want true", effort)
			}
		})
	}
	for _, effort := range []string{"", "MAX", "ultra", "automatic"} {
		t.Run("Should reject "+effort, func(t *testing.T) {
			t.Parallel()

			if IsValid(effort) {
				t.Fatalf("IsValid(%q) = true, want false", effort)
			}
		})
	}

	t.Run("Should expose the canonical values in display order", func(t *testing.T) {
		t.Parallel()

		want := []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}
		if got := Values(); !slices.Equal(got, want) {
			t.Fatalf("Values() = %#v, want %#v", got, want)
		}
	})
}

func TestInvalidEffortError(t *testing.T) {
	t.Parallel()

	t.Run("Should describe the invalid value and canonical choices", func(t *testing.T) {
		t.Parallel()

		err := (&InvalidEffortError{Path: "reasoning_effort", Value: " ultra "}).Error()
		want := "reasoning_effort \"ultra\" is invalid; expected none, minimal, low, medium, high, xhigh, max"
		if err != want {
			t.Fatalf("InvalidEffortError.Error() = %q, want %q", err, want)
		}
	})

	t.Run("Should tolerate a nil receiver", func(t *testing.T) {
		t.Parallel()

		var err *InvalidEffortError
		if got, want := err.Error(), "invalid reasoning effort"; got != want {
			t.Fatalf("InvalidEffortError.Error() = %q, want %q", got, want)
		}
	})
}
