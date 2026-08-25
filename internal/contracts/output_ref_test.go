package contracts

import (
	"encoding/json"
	"testing"
)

func TestOutputReference(t *testing.T) {
	t.Parallel()

	t.Run("Should produce and recognize a canonical content digest", func(t *testing.T) {
		t.Parallel()

		ref := OutputRefForPayload(json.RawMessage(`{"answer":42}`))
		if !OutputRefLooksContentAddressed(ref) {
			t.Fatalf("OutputRefForPayload() = %q, want recognized digest", ref)
		}
		if OutputRefLooksContentAddressed("sha256:not-a-digest") {
			t.Fatal("OutputRefLooksContentAddressed(invalid) = true")
		}
	})
}
