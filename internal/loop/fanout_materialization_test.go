package loop

import (
	"errors"
	"reflect"
	"testing"
)

// Invariant: persisted fan-out chunks retain each item's original order and value when decoded.
func TestParseFanOutMaterializationShouldDecodePersistedPayloads(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize pre-candidate chunks", func(t *testing.T) {
		t.Parallel()

		materialization, ok, err := parseFanOutMaterialization(
			`{"kind":"fan_out","branches":2,"batch_size":2,"max_parallel":3,"chunks":[["alpha",{"id":"B"}],[true]]}`,
		)
		if err != nil {
			t.Fatalf("parseFanOutMaterialization() error = %v", err)
		}
		if !ok {
			t.Fatal("parseFanOutMaterialization() ok = false, want true")
		}
		want := [][]fanOutCandidate{
			{{Index: 0, Item: "alpha"}, {Index: 1, Item: map[string]any{"id": "B"}}},
			{{Index: 2, Item: true}},
		}
		if !reflect.DeepEqual(materialization.Chunks, want) {
			t.Fatalf("materialization chunks = %#v, want %#v", materialization.Chunks, want)
		}
	})

	t.Run("Should reject malformed chunks", func(t *testing.T) {
		t.Parallel()

		_, ok, err := parseFanOutMaterialization(
			`{"kind":"fan_out","branches":1,"batch_size":1,"max_parallel":1,"chunks":[["alpha",]]}`,
		)
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("parseFanOutMaterialization() error = %v, want ErrValidation", err)
		}
		if ok {
			t.Fatal("parseFanOutMaterialization() ok = true, want false")
		}
	})
}
