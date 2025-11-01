package testutil

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

// AssertConfigEqual fails the test when want and got differ, emitting a stable JSON diff for easier debugging.
func AssertConfigEqual(t *testing.T, want, got any) {
	t.Helper()
	if reflect.DeepEqual(want, got) {
		return
	}
	serializedWant := mustMarshalConfig(want)
	serializedGot := mustMarshalConfig(got)
	t.Fatalf("config mismatch\nwant: %s\n got: %s", serializedWant, serializedGot)
}

func mustMarshalConfig(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%+v", v)
	}
	return string(data)
}
