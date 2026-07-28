package bridges

import "testing"

func TestSemanticJSONEqualTreatsEquivalentNumbersAsEqual(t *testing.T) {
	t.Parallel()

	t.Run("Should treat numerically equivalent literals as equal", func(t *testing.T) {
		t.Parallel()

		if !semanticJSONEqual(
			[]byte(`{"value":1,"nested":[{"ratio":1e1}]}`),
			[]byte(`{"value":1.0,"nested":[{"ratio":10.0}]}`),
		) {
			t.Fatal("semanticJSONEqual() = false, want true for equivalent numeric values")
		}
	})

	t.Run("Should reject multiple concatenated JSON values", func(t *testing.T) {
		t.Parallel()

		if semanticJSONEqual([]byte(`{"value":1}{"value":1}`), []byte(`{"value":1}`)) {
			t.Fatal("semanticJSONEqual() = true, want false for multiple concatenated JSON values")
		}
	})
}
