package modelcatalog

import (
	"slices"
	"testing"
)

func TestReasoningSourceVocabulary(t *testing.T) {
	t.Parallel()

	t.Run("Should expose canonical reasoning sources", func(t *testing.T) {
		t.Parallel()

		want := []string{"acp", "catalog"}
		if got := ReasoningSourceValues(); !slices.Equal(got, want) {
			t.Fatalf("ReasoningSourceValues() = %#v, want %#v", got, want)
		}
	})
}
