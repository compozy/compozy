package daemon

import (
	"strings"
	"testing"

	looppkg "github.com/compozy/agh/internal/loop"
)

func TestLoopRunPayloadShouldPropagateMapCloneErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should reject non-serializable loop run inputs", func(t *testing.T) {
		t.Parallel()

		_, err := loopRunPayload(looppkg.Run{
			ID:     "run-1",
			Inputs: map[string]any{"bad": func() {}},
		})
		if err == nil {
			t.Fatal("loopRunPayload() error = nil, want clone error")
		}
		if !strings.Contains(err.Error(), "clone loop api map") {
			t.Fatalf("loopRunPayload() error = %v, want clone loop api map", err)
		}
	})
}
