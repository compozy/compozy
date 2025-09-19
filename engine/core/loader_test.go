package core

import (
	"testing"
)

func TestRejectDollarKeys_AllowsSchemaContexts(t *testing.T) {
	y := []byte("input:\n  schema:\n    $schema: http://json-schema.org/draft-07/schema#\n    type: object\n")
	if err := rejectDollarKeys(y, "test.yaml"); err != nil {
		t.Fatalf("unexpected error allowing $ in schema context: %v", err)
	}
}

func TestRejectDollarKeys_RejectsNonSchema(t *testing.T) {
	y := []byte("$ref: something")
	if err := rejectDollarKeys(y, "test.yaml"); err == nil {
		t.Fatalf("expected error for $ at root outside schema context")
	}
}
