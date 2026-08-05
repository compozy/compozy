package acp

import (
	"slices"
	"testing"
)

func TestParseCommandString(t *testing.T) {
	t.Parallel()

	command, args, err := parseCommandString(`npx -y "agent client" --flag='hello world'`)
	if err != nil {
		t.Fatalf("parseCommandString() error = %v", err)
	}
	if command != "npx" {
		t.Fatalf("parseCommandString() command = %q, want %q", command, "npx")
	}
	wantArgs := []string{"-y", "agent client", "--flag=hello world"}
	if !slices.Equal(args, wantArgs) {
		t.Fatalf("parseCommandString() args = %#v, want %#v", args, wantArgs)
	}
}
