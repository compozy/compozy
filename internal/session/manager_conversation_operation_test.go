package session

import (
	"errors"
	"testing"
)

func TestConversationOperationLockRejectsSameOwnerAcquisition(t *testing.T) {
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
}
