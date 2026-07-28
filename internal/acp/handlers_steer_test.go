package acp

import (
	"context"
	"testing"
	"time"
)

func TestSteerConsumeContext(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve parent deadlines after detaching", func(t *testing.T) {
		t.Parallel()

		parent, cancelParent := context.WithDeadline(context.Background(), time.Now().Add(time.Minute))
		defer cancelParent()

		consumeCtx, cancel := steerConsumeContext(parent, nil)
		defer cancel()

		deadline, ok := consumeCtx.Deadline()
		if !ok {
			t.Fatal("consumeCtx.Deadline() missing, want preserved parent deadline")
		}
		parentDeadline, _ := parent.Deadline()
		if !deadline.Equal(parentDeadline) {
			t.Fatalf("deadline = %v, want %v", deadline, parentDeadline)
		}

		cancelParent()
		select {
		case <-consumeCtx.Done():
			t.Fatalf("consumeCtx canceled after parent cancel: %v", consumeCtx.Err())
		default:
		}
	})

	t.Run("Should add a fallback timeout when parent has no deadline", func(t *testing.T) {
		t.Parallel()

		parent, cancelParent := context.WithCancel(context.Background())
		defer cancelParent()

		consumeCtx, cancel := steerConsumeContext(parent, nil)
		defer cancel()

		deadline, ok := consumeCtx.Deadline()
		if !ok {
			t.Fatal("consumeCtx.Deadline() missing, want fallback timeout")
		}
		if remaining := time.Until(deadline); remaining <= 0 || remaining > steerDispatchTimeout {
			t.Fatalf("remaining timeout = %v, want 0 < remaining <= %v", remaining, steerDispatchTimeout)
		}

		cancelParent()
		select {
		case <-consumeCtx.Done():
			t.Fatalf("consumeCtx canceled after parent cancel: %v", consumeCtx.Err())
		default:
		}
	})
}

func TestSteerTimeoutContext(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve an existing deadline", func(t *testing.T) {
		t.Parallel()

		parent, cancelParent := context.WithDeadline(context.Background(), time.Now().Add(time.Minute))
		defer cancelParent()

		dispatchCtx, cancel := steerTimeoutContext(parent)
		defer cancel()

		deadline, ok := dispatchCtx.Deadline()
		if !ok {
			t.Fatal("dispatchCtx.Deadline() missing, want preserved deadline")
		}
		parentDeadline, _ := parent.Deadline()
		if !deadline.Equal(parentDeadline) {
			t.Fatalf("deadline = %v, want %v", deadline, parentDeadline)
		}
	})

	t.Run("Should add a timeout when no deadline exists", func(t *testing.T) {
		t.Parallel()

		dispatchCtx, cancel := steerTimeoutContext(context.Background())
		defer cancel()

		deadline, ok := dispatchCtx.Deadline()
		if !ok {
			t.Fatal("dispatchCtx.Deadline() missing, want timeout")
		}
		if remaining := time.Until(deadline); remaining <= 0 || remaining > steerDispatchTimeout {
			t.Fatalf("remaining timeout = %v, want 0 < remaining <= %v", remaining, steerDispatchTimeout)
		}
	})
}
