package recovery

import (
	"errors"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
)

func TestFilterJobsBySafeName(t *testing.T) {
	t.Parallel()

	jobs := []model.Job{
		{SafeName: "already-green"},
		{SafeName: "needs-restart"},
		{SafeName: "also-failed"},
	}

	t.Run("Should preserve prepared order for failed SafeNames", func(t *testing.T) {
		t.Parallel()

		filtered, err := FilterJobsBySafeName(jobs, []string{" also-failed ", "needs-restart"})
		if err != nil {
			t.Fatalf("FilterJobsBySafeName() error = %v", err)
		}
		if got := joinedSafeNames(filtered); got != "needs-restart,also-failed" {
			t.Fatalf("filtered jobs = %s, want needs-restart,also-failed", got)
		}
	})

	t.Run("Should reject empty failed IDs", func(t *testing.T) {
		t.Parallel()

		_, err := FilterJobsBySafeName(jobs, []string{" ", ""})
		if !errors.Is(err, errNoFailedJobIDs) {
			t.Fatalf("FilterJobsBySafeName() error = %v, want errNoFailedJobIDs", err)
		}
	})

	t.Run("Should reject stale failed IDs", func(t *testing.T) {
		t.Parallel()

		_, err := FilterJobsBySafeName(jobs, []string{"needs-restart", "missing"})
		if !errors.Is(err, errFailedJobNotPrepared) || !strings.Contains(err.Error(), `"missing"`) {
			t.Fatalf("FilterJobsBySafeName() error = %v, want errFailedJobNotPrepared for missing", err)
		}
	})

	t.Run("Should reject when no prepared jobs match", func(t *testing.T) {
		t.Parallel()

		_, err := FilterJobsBySafeName(jobs, []string{"missing"})
		if !errors.Is(err, errNoPreparedJobsMatched) || !strings.Contains(err.Error(), "missing") {
			t.Fatalf("FilterJobsBySafeName() error = %v, want errNoPreparedJobsMatched", err)
		}
	})
}

func joinedSafeNames(jobs []model.Job) string {
	names := make([]string, 0, len(jobs))
	for i := range jobs {
		names = append(names, jobs[i].SafeName)
	}
	return strings.Join(names, ",")
}
