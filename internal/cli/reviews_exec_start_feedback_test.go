package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	request apicore.ReviewRunRequest
}

func (c *blockingReviewStartClient) StartReviewRun(
	ctx context.Context,
	_ string,
	_ string,
	_ int,
	req apicore.ReviewRunRequest,
) (apicore.Run, error) {
	c.request = req
	<-ctx.Done()
	return apicore.Run{}, ctx.Err()
}

type lostReviewStartResponseClient struct {
	*stubDaemonCommandClient
	durableRun apicore.Run
}

func (c *lostReviewStartResponseClient) StartReviewRun(
	ctx context.Context,
	_ string,
	_ string,
	_ int,
	req apicore.ReviewRunRequest,
) (apicore.Run, error) {
	runID, err := testReviewRunID(req)
	if err != nil {
		return apicore.Run{}, err
	}
	c.durableRun = apicore.Run{RunID: runID, Status: "starting"}
	<-ctx.Done()
	return apicore.Run{}, ctx.Err()
}

func (c *lostReviewStartResponseClient) GetRun(ctx context.Context, runID string) (apicore.Run, error) {
	if err := ctx.Err(); err != nil {
		return apicore.Run{}, fmt.Errorf("recovery context: %w", err)
	}
	if runID != c.durableRun.RunID {
		return apicore.Run{}, fmt.Errorf("get run %q: want %q", runID, c.durableRun.RunID)
	}
	return c.durableRun, nil
}

type echoReviewStartClient struct {
	*stubDaemonCommandClient
	request apicore.ReviewRunRequest
}

func (c *echoReviewStartClient) StartReviewRun(
	_ context.Context,
	_ string,
	_ string,
	_ int,
	req apicore.ReviewRunRequest,
) (apicore.Run, error) {
	c.request = req
	runID, err := testReviewRunID(req)
	if err != nil {
		return apicore.Run{}, err
	}
	return apicore.Run{RunID: runID}, nil
}

func testReviewRunID(req apicore.ReviewRunRequest) (string, error) {
	var overrides daemonRuntimeOverrides
	if err := json.Unmarshal(req.RuntimeOverrides, &overrides); err != nil {
		return "", fmt.Errorf("decode review runtime overrides: %w", err)
	}
	if overrides.RunID == nil || strings.TrimSpace(*overrides.RunID) == "" {
		return "", errors.New("review runtime overrides are missing run_id")
	}
	return strings.TrimSpace(*overrides.RunID), nil
}

func TestStartReviewRunWithFeedback(t *testing.T) {
	t.Run("Should stay quiet and return the run on the fast common path", func(t *testing.T) {
		t.Parallel()
		var status bytes.Buffer
		client := &echoReviewStartClient{stubDaemonCommandClient: &stubDaemonCommandClient{}}
		run, err := startReviewRunWithFeedback(
			context.Background(), &status, client, "root", "wf", 1, apicore.ReviewRunRequest{},
			time.Second, time.Minute,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		requestRunID, requestErr := testReviewRunID(client.request)
		if requestErr != nil {
			t.Fatalf("request run id: %v", requestErr)
		}
		if run.RunID != requestRunID || !strings.HasPrefix(run.RunID, "reviews-wf-") {
			t.Fatalf("run id = %q, request run id = %q, want matching generated review id", run.RunID, requestRunID)
		}
		if status.Len() != 0 {
			t.Fatalf("expected a quiet fast path, got %q", status.String())
		}
	})
	t.Run("Should surface preparing feedback and an actionable timeout when the start blocks", func(t *testing.T) {
		t.Parallel()
		var status bytes.Buffer
		client := &blockingReviewStartClient{
			stubDaemonCommandClient: &stubDaemonCommandClient{getRunErr: errors.New("run not found")},
		}
		_, err := startReviewRunWithFeedback(
			context.Background(), &status, client, "root", "wf", 1, apicore.ReviewRunRequest{},
			120*time.Millisecond, 10*time.Millisecond,
		)
		if err == nil {
			t.Fatal("expected a timeout error, got nil")
		}
		if !strings.Contains(err.Error(), "outcome was not confirmed within") {
			t.Fatalf("error missing timeout context: %v", err)
		}
		requestRunID, requestErr := testReviewRunID(client.request)
		if requestErr != nil {
			t.Fatalf("request run id: %v", requestErr)
		}
		if !strings.Contains(err.Error(), requestRunID) ||
			!strings.Contains(err.Error(), "compozy runs watch "+requestRunID) {
			t.Fatalf("error missing stable recovery handle: %v", err)
		}
		if strings.Contains(err.Error(), "before creating a durable run") {
			t.Fatalf("error makes an unsafe pre-commit claim: %v", err)
		}
		if !strings.Contains(status.String(), "Preparing review run") {
			t.Fatalf("expected preparing feedback on a slow start, got %q", status.String())
		}
	})
	t.Run("Should recover the durable run when the start response loses the deadline race", func(t *testing.T) {
		t.Parallel()
		var status bytes.Buffer
		client := &lostReviewStartResponseClient{stubDaemonCommandClient: &stubDaemonCommandClient{}}
		run, err := startReviewRunWithFeedback(
			context.Background(), &status, client, "root", "wf", 1, apicore.ReviewRunRequest{},
			120*time.Millisecond, time.Minute,
		)
		if err != nil {
			t.Fatalf("recover durable review run: %v", err)
		}
		if run.RunID == "" || run.RunID != client.durableRun.RunID {
			t.Fatalf("recovered run id = %q, durable run id = %q", run.RunID, client.durableRun.RunID)
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
