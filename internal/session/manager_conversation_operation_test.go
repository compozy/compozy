package session

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestConversationOperationLockRejectsSameOwnerAcquisition(t *testing.T) {
	t.Parallel()

	t.Run("Should reject same-owner acquisition", func(t *testing.T) {
		t.Parallel()

		manager := &Manager{}
		operationCtx, unlock, err := manager.lockConversationOperation(t.Context(), "sess-1")
		if err != nil {
			t.Fatalf("lockConversationOperation() error = %v", err)
		}
		defer unlock()

		_, _, err = manager.lockConversationOperation(operationCtx, "sess-1")
		if !errors.Is(err, errConversationOperationReentrant) {
			t.Fatalf("nested lockConversationOperation() error = %v, want reentrant error", err)
		}
	})
}

func TestDeriveConversationLock(t *testing.T) {
	t.Parallel()
	t.Run("Should time out a busy source and preserve caller cancellation", func(t *testing.T) {
		t.Parallel()
		manager := &Manager{}
		_, unlock, err := manager.lockConversationOperation(t.Context(), "sess-1")
		if err != nil {
			t.Fatal(err)
		}
		defer unlock()
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()
		if _, err := manager.lockDeriveConversation(ctx, "sess-1"); !errors.Is(err, ErrDeriveSourceBusy) {
			t.Fatalf("lockDeriveConversation(busy) = %v, want source busy before the caller deadline", err)
		}
		cancel()
		if _, err := manager.lockDeriveConversation(ctx, "sess-1"); !errors.Is(err, context.Canceled) {
			t.Fatalf("lockDeriveConversation(canceled) = %v, want caller cancellation", err)
		}
	})
}
