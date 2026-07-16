package recovery

import "testing"

func TestParkedOutcomeIsNotCanceled(t *testing.T) {
	t.Parallel()

	t.Run("Should not treat a parked run as canceled", func(t *testing.T) {
		t.Parallel()
		outcome := RunOutcome{Status: StatusParked}
		if outcome.Canceled() {
			t.Fatal("RunOutcome{Status: parked}.Canceled() = true, want false")
		}
	})

	t.Run("Should not treat a run with a parked job as canceled", func(t *testing.T) {
		t.Parallel()
		outcome := RunOutcome{
			Status: StatusFailed,
			Jobs: []JobOutcome{
				{SafeName: "task_01", Status: StatusSucceeded},
				{SafeName: "task_02", Status: StatusParked},
			},
		}
		if outcome.Canceled() {
			t.Fatal("RunOutcome with a parked job Canceled() = true, want false")
		}
	})

	t.Run("Should keep parked distinct from the canceled and failed vocabularies", func(t *testing.T) {
		t.Parallel()
		if StatusParked == StatusCanceled {
			t.Fatal("StatusParked must be distinct from StatusCanceled")
		}
		if StatusParked == StatusFailed {
			t.Fatal("StatusParked must be distinct from StatusFailed")
		}
		if StatusParked != "parked" {
			t.Fatalf("StatusParked = %q, want %q", StatusParked, "parked")
		}
	})

	t.Run("Should not report a parked job as a failed job for recovery", func(t *testing.T) {
		t.Parallel()
		outcome := RunOutcome{
			Status: StatusFailed,
			Jobs:   []JobOutcome{{SafeName: "task_02", Status: StatusParked}},
		}
		if ids := outcome.FailedJobIDs(); len(ids) != 0 {
			t.Fatalf("FailedJobIDs() = %#v, want empty for a parked job", ids)
		}
	})
}
