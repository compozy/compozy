package recovery

import (
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
		if err == nil || !strings.Contains(err.Error(), "no failed job IDs") {
			t.Fatalf("FilterJobsBySafeName() error = %v, want empty failed IDs error", err)
		}
	})

	t.Run("Should reject stale failed IDs", func(t *testing.T) {
		t.Parallel()

		_, err := FilterJobsBySafeName(jobs, []string{"needs-restart", "missing"})
		if err == nil || !strings.Contains(err.Error(), `failed job "missing"`) {
			t.Fatalf("FilterJobsBySafeName() error = %v, want stale failed ID error", err)
		}
	})

	t.Run("Should reject when no prepared jobs match", func(t *testing.T) {
		t.Parallel()

		_, err := FilterJobsBySafeName(jobs, []string{"missing"})
		if err == nil || !strings.Contains(err.Error(), "no prepared jobs matched") {
			t.Fatalf("FilterJobsBySafeName() error = %v, want no match error", err)
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
