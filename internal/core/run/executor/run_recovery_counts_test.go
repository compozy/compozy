package executor

import "testing"

// TestCountRunRecovery pins the printed end-of-run summary buckets: a stalled job
// that later succeeded is a recovery, a plain success is not, and a parked job is
// neither a success nor a plain failure.
func TestCountRunRecovery(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		jobs  []job
		total int
		want  runRecoveryCounts
	}{
		{
			name:  "plain completions report no recovery",
			jobs:  []job{{Status: runStatusSucceeded}, {Status: runStatusSucceeded}},
			total: 2,
			want:  runRecoveryCounts{total: 2, succeeded: 2},
		},
		{
			name:  "a stalled-then-succeeded job is recovered and still succeeded",
			jobs:  []job{{Status: runStatusSucceeded}, {Status: runStatusSucceeded, Stalled: true}},
			total: 2,
			want:  runRecoveryCounts{total: 2, succeeded: 2, recovered: 1},
		},
		{
			name:  "a parked job is neither succeeded nor failed",
			jobs:  []job{{Status: runStatusSucceeded}, {Status: runStatusParked, Stalled: true}},
			total: 2,
			want:  runRecoveryCounts{total: 2, succeeded: 1, parked: 1},
		},
		{
			name: "one recovered and one parked across a batch",
			jobs: []job{
				{Status: runStatusSucceeded},
				{Status: runStatusSucceeded, Stalled: true},
				{Status: runStatusParked, Stalled: true},
			},
			total: 3,
			want:  runRecoveryCounts{total: 3, succeeded: 2, recovered: 1, parked: 1},
		},
		{
			name:  "failed absorbs every job that neither succeeded nor parked",
			jobs:  []job{{Status: runStatusSucceeded}, {Status: runStatusFailed}, {Status: runStatusCanceled}},
			total: 3,
			want:  runRecoveryCounts{total: 3, succeeded: 1, failed: 2},
		},
		{
			name:  "a stalled job that never succeeded is not recovered",
			jobs:  []job{{Status: runStatusFailed, Stalled: true}},
			total: 1,
			want:  runRecoveryCounts{total: 1, failed: 1},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := countRunRecovery(tc.jobs, tc.total); got != tc.want {
				t.Fatalf("countRunRecovery() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
