package taskgroups

import "testing"

// Suite: selection fingerprint
// Invariant: identical initiative, task-group set, and plan checksum always produce one stable fingerprint.
// Boundary IN: pure selection fingerprint computation
// Boundary OUT: durable run persistence and daemon re-launch behavior

func TestSelectionFingerprintOrderIndependentUT040(t *testing.T) {
	t.Parallel()

	left := SelectionFingerprint("initiative", []string{"TG-002", "TG-001"}, "plan-checksum")
	right := SelectionFingerprint("initiative", []string{"TG-001", "TG-002"}, "plan-checksum")
	if left != right {
		t.Fatalf("SelectionFingerprint() order changed result: %q != %q", left, right)
	}
}

func TestSelectionFingerprintDeterministicUT041(t *testing.T) {
	t.Parallel()

	first := SelectionFingerprint("initiative", []string{"TG-001", "TG-002"}, "plan-checksum")
	second := SelectionFingerprint("initiative", []string{"TG-001", "TG-002"}, "plan-checksum")
	if first != second {
		t.Fatalf("SelectionFingerprint() repeated result: %q != %q", first, second)
	}
}

func TestSelectionFingerprintChangesWithInputUT042(t *testing.T) {
	t.Parallel()

	baseline := SelectionFingerprint("initiative", []string{"TG-001", "TG-002"}, "plan-checksum")
	tests := []struct {
		name       string
		initiative string
		groupIDs   []string
		checksum   string
	}{
		{
			name:       "plan checksum",
			initiative: "initiative",
			groupIDs:   []string{"TG-001", "TG-002"},
			checksum:   "changed-checksum",
		},
		{
			name:       "initiative",
			initiative: "other-initiative",
			groupIDs:   []string{"TG-001", "TG-002"},
			checksum:   "plan-checksum",
		},
		{
			name:       "task group set",
			initiative: "initiative",
			groupIDs:   []string{"TG-001", "TG-003"},
			checksum:   "plan-checksum",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := SelectionFingerprint(tt.initiative, tt.groupIDs, tt.checksum)
			if got == baseline {
				t.Fatalf("SelectionFingerprint() did not change for %s", tt.name)
			}
		})
	}
}
