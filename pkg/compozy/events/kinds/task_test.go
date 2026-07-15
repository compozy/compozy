// Suite: Parallel task status semantics.
// Invariant: Only canonical merged and recovered statuses report integrated content.
// Boundary IN: TaskParallelTaskStatus values and IsIntegrated.
// Boundary OUT: Event transport and UI rendering, covered by their package suites.
package kinds

import "testing"

func TestTaskParallelTaskStatusIsIntegrated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status TaskParallelTaskStatus
		want   bool
	}{
		{name: "Should report merged as integrated", status: TaskParallelTaskStatusMerged, want: true},
		{name: "Should report recovered as integrated", status: TaskParallelTaskStatusRecovered, want: true},
		{name: "Should reject failed", status: TaskParallelTaskStatusFailed},
		{name: "Should reject skipped", status: TaskParallelTaskStatusSkipped},
		{name: "Should reject canceled", status: TaskParallelTaskStatusCanceled},
		{name: "Should reject unknown", status: TaskParallelTaskStatus("unknown")},
		{name: "Should reject empty", status: TaskParallelTaskStatus("")},
		{name: "Should reject non-canonical whitespace", status: TaskParallelTaskStatus(" merged ")},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.status.IsIntegrated(); got != tc.want {
				t.Fatalf("IsIntegrated() = %t, want %t", got, tc.want)
			}
		})
	}
}
