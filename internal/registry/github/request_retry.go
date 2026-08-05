package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/retry"
)

type githubRequestAttemptError struct {
	cause     error
	response  *http.Response
	retryable bool
}

func (e *githubRequestAttemptError) Error() string {
	return e.cause.Error()
}

func (e *githubRequestAttemptError) Unwrap() error {
	return e.cause
}

func (c *Client) requestRetryPolicy() retry.Policy {
	return retry.Policy{
		MaxAttempts: c.maxRetries + 1,
		Delay: retry.Exponential(retry.ExponentialConfig{
			BaseDelay: c.initialBackoff,
			MaxDelay:  c.maxBackoff,
		}),
		Sleep: func(ctx context.Context, delay time.Duration) error {
			if err := c.sleep(ctx, delay); err != nil {
				return &githubRequestAttemptError{
					cause: fmt.Errorf("github: retry wait aborted: %w", err),
				}
			}
			return nil
		},
		OnRetry: func(_ context.Context, attempt retry.Attempt) error {
			attemptErr, ok := errors.AsType[*githubRequestAttemptError](attempt.Err)
			if ok && attemptErr.response != nil {
				c.prepareRetryResponse(attemptErr.response, attempt.Number-1)
			}
			return nil
		},
	}
}

func newGitHubTransportAttemptError(ctx context.Context, err error) *githubRequestAttemptError {
	retryable := !errors.Is(err, context.Canceled) &&
		!errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil
	return &githubRequestAttemptError{
		cause:     fmt.Errorf("github: request failed: %w", err),
		retryable: retryable,
	}
}

func retryGitHubRequest(err error) bool {
	attemptErr, ok := errors.AsType[*githubRequestAttemptError](err)
	return ok && attemptErr.retryable
}

func finishGitHubRequest(ctx context.Context, err error) (*http.Response, error) {
	attemptErr, ok := errors.AsType[*githubRequestAttemptError](err)
	if ok {
		if attemptErr.response != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				cleanupErr := closeResponseBody(attemptErr.response.Body, "canceled retry response")
				return nil, errors.Join(
					attemptErr.cause,
					fmt.Errorf("github: request failed: %w", ctxErr),
					cleanupErr,
				)
			}
			return attemptErr.response, nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil && !errors.Is(attemptErr.cause, ctxErr) {
			return nil, errors.Join(attemptErr.cause, fmt.Errorf("github: request failed: %w", ctxErr))
		}
		return nil, attemptErr.cause
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, fmt.Errorf("github: request failed: %w", ctxErr)
	}
	return nil, err
}
