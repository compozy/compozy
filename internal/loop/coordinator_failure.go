package loop

import (
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/task"
)

const (
	coordinatorFailureCodeWatchPoll = "watch_poll_failed"
	coordinatorFailureWatchCause    = "The watch source failed before it could produce a generation."
	coordinatorFailureWatchRecovery = "Verify the Loop watch provider and workspace prerequisites, then start a new run."
)

type coordinatorFailureError struct {
	cause   error
	failure task.RunFailure
}

func newWatchPollFailureError(cause error) error {
	failure, err := task.NewCoordinatorRunFailure(
		coordinatorFailureCodeWatchPoll,
		coordinatorFailureWatchCause,
		coordinatorFailureWatchRecovery,
	)
	if err != nil {
		return errors.Join(cause, err)
	}
	return &coordinatorFailureError{cause: cause, failure: failure}
}

func (e *coordinatorFailureError) Error() string {
	return fmt.Sprintf("loop coordinator: %v", e.cause)
}

func (e *coordinatorFailureError) Unwrap() error {
	return e.cause
}

func (e *coordinatorFailureError) SafeRunFailure() task.RunFailure {
	return task.RunFailure{
		Error:    e.failure.Error,
		Metadata: cloneRawMessage(e.failure.Metadata),
	}
}
