package terminal

import "testing"

func TestRecordingAvailable(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		interactive bool
	}{
		{name: "Should enable recording for interactive terminals", interactive: true},
		{name: "Should omit recording for execute-only terminals", interactive: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := RecordingAvailable(Capabilities{Interactive: tc.interactive}); got != tc.interactive {
				t.Fatalf("RecordingAvailable() = %t, want %t", got, tc.interactive)
			}
		})
	}
}
