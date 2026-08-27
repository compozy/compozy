package calls

import (
	"testing"
)

// Suite: bound-child prompt contract
// Invariant: every child prompt names its call, terminal settlement duty, CWR consequence, and delegation budget.
// Boundary IN: CallPromptWithRemainingDepth.
// Boundary OUT: runtime prompt transport is owned by the daemon call runtime suite.
func TestCallPromptStatesBoundChildDuty(t *testing.T) {
	t.Parallel()
	const callID = "call_01JBD8G2K7Q9"
	for _, test := range []struct {
		name       string
		remaining  int
		delegation string
	}{
		{name: "Should state two remaining levels", remaining: 2, delegation: "You may delegate 2 more levels."},
		{name: "Should state one remaining level", remaining: 1, delegation: "You may delegate 1 more level."},
		{name: "Should state that delegation is exhausted", remaining: 0, delegation: "You cannot delegate further."},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := CallPromptWithRemainingDepth(callID, "Review this.", test.remaining)
			want := "Bound call: " + callID + "\n" +
				"Duty: compozy__call_return is your terminal act. agent_call, mailbox messages, and ordinary prose do not " +
				"settle this call; use agent_call only for further delegation; a truly empty omitted return settles " +
				"completed-without-result.\n" +
				"Delegation: " + test.delegation + "\n\n" +
				"Assignment:\nReview this."
			if got != want {
				t.Fatalf("CallPromptWithRemainingDepth(%d) = %q, want %q", test.remaining, got, want)
			}
		})
	}
}
