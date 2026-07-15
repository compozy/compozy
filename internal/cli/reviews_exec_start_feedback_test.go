package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
)

// blockingReviewStartClient blocks StartReviewRun until the passed context is
// canceled, simulating a daemon whose synchronous workflow sync is stuck on the
// single-writer global.db lock.
type blockingReviewStartClient struct {
	*stubDaemonCommandClient
}

func (c *blockingReviewStartClient) StartReviewRun(
	ctx context.Context,
	_ string,
	_ string,
	_ int,
	_ apicore.ReviewRunRequest,
) (apicore.Run, error) {
	<-ctx.Done()
	return apicore.Run{}, ctx.Err()
}

func TestStartReviewRunWithFeedback(t *testing.T) {
	t.Run("Should stay quiet and return the run on the fast common path", func(t *testing.T) {
		t.Parallel()
		var status bytes.Buffer
		client := &stubDaemonCommandClient{reviewRun: apicore.Run{RunID: "run-1"}}
		run, err := startReviewRunWithFeedback(
			context.Background(), &status, client, "root", "wf", 1, apicore.ReviewRunRequest{},
			time.Second, time.Minute,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if run.RunID != "run-1" {
			t.Fatalf("run id = %q, want run-1", run.RunID)
		}
		if status.Len() != 0 {
			t.Fatalf("expected a quiet fast path, got %q", status.String())
		}
	})
	t.Run("Should surface preparing feedback and an actionable timeout when the start blocks", func(t *testing.T) {
		t.Parallel()
		var status bytes.Buffer
		client := &blockingReviewStartClient{stubDaemonCommandClient: &stubDaemonCommandClient{}}
		_, err := startReviewRunWithFeedback(
			context.Background(), &status, client, "root", "wf", 1, apicore.ReviewRunRequest{},
			120*time.Millisecond, 10*time.Millisecond,
		)
		if err == nil {
			t.Fatal("expected a timeout error, got nil")
		}
		if !strings.Contains(err.Error(), "did not start within") {
			t.Fatalf("error missing timeout context: %v", err)
		}
		if !strings.Contains(err.Error(), "before creating a durable run") {
			t.Fatalf("error missing canceled-start guarantee: %v", err)
		}
		if !strings.Contains(err.Error(), "retry after workflow sync contention clears") {
			t.Fatalf("error missing actionable hint: %v", err)
		}
		if !strings.Contains(status.String(), "Preparing review run") {
			t.Fatalf("expected preparing feedback on a slow start, got %q", status.String())
		}
	})
	t.Run("Should propagate a non-timeout daemon error unchanged", func(t *testing.T) {
		t.Parallel()
		var status bytes.Buffer
		boom := errors.New("start rejected")
		client := &stubDaemonCommandClient{reviewRunErr: boom}
		_, err := startReviewRunWithFeedback(
			context.Background(), &status, client, "root", "wf", 1, apicore.ReviewRunRequest{},
			time.Second, time.Minute,
		)
		if !errors.Is(err, boom) {
			t.Fatalf("expected the daemon error to propagate, got %v", err)
		}
	})
	t.Run("Should not misreport a caller cancellation as a start timeout", func(t *testing.T) {
		t.Parallel()
		var status bytes.Buffer
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		client := &blockingReviewStartClient{stubDaemonCommandClient: &stubDaemonCommandClient{}}
		_, err := startReviewRunWithFeedback(
			ctx, &status, client, "root", "wf", 1, apicore.ReviewRunRequest{},
			time.Minute, time.Minute,
		)
		if err == nil {
			t.Fatal("expected a cancellation error, got nil")
		}
		if strings.Contains(err.Error(), "did not start within") {
			t.Fatalf("caller cancellation misreported as start timeout: %v", err)
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})
}
