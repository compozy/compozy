package cli

import (
	"strings"
	"testing"
)

func TestExactOneNonBlankArgRejectsBlankArgument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "Should reject blank task id",
			args: []string{"task", "get", ""},
		},
		{
			name: "Should reject whitespace session id",
			args: []string{"session", "status", "   "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := executeRootCommand(t, newTestDeps(t, &stubClient{}), tt.args...)
			if err == nil || !strings.Contains(err.Error(), "cannot be blank") {
				t.Fatalf("executeRootCommand(%v) error = %v, want cannot be blank", tt.args, err)
			}
		})
	}
}
