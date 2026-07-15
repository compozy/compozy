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
		{name: "merged", status: TaskParallelTaskStatusMerged, want: true},
		{name: "recovered", status: TaskParallelTaskStatusRecovered, want: true},
		{name: "failed", status: TaskParallelTaskStatusFailed},
		{name: "skipped", status: TaskParallelTaskStatusSkipped},
		{name: "canceled", status: TaskParallelTaskStatusCanceled},
		{name: "unknown", status: TaskParallelTaskStatus("unknown")},
		{name: "empty", status: TaskParallelTaskStatus("")},
		{name: "non-canonical whitespace", status: TaskParallelTaskStatus(" merged ")},
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
