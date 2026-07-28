package hooks

import "testing"

func TestPermissionDecisionDenied(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		decision string
		want     bool
	}{
		{name: "empty", decision: "", want: false},
		{name: "allow once", decision: "allow-once", want: false},
		{name: "allow always", decision: "allow-always", want: false},
		{name: "pending", decision: "pending", want: false},
		{name: "deny", decision: "deny", want: true},
		{name: "deny once", decision: "deny-once", want: true},
		{name: "reject", decision: "reject", want: true},
		{name: "reject once", decision: "reject-once", want: true},
		{name: "reject always", decision: "reject-always", want: true},
		{name: "blocked", decision: "blocked", want: true},
		{name: "trim and case fold", decision: " Reject-Once ", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := permissionDecisionDenied(tt.decision); got != tt.want {
				t.Fatalf("permissionDecisionDenied(%q) = %v, want %v", tt.decision, got, tt.want)
			}
		})
	}
}
