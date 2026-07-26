package core

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

const networkChannelRollbackTimeout = 5 * time.Second

// rollbackCreatedNetworkChannel compensates every resource provisioned before
// channel creation failed. Cleanup must survive request cancellation while
// remaining bounded, and channel authority is removed only after all created
// sessions have received their stop request.
func rollbackCreatedNetworkChannel(
	requestCtx context.Context,
	sessions SessionManager,
	networkStore NetworkStore,
	ref store.NetworkChannelRef,
	createdIDs []string,
	baseErr error,
) error {
	rollbackCtx, cancel := context.WithTimeout(
		context.WithoutCancel(requestCtx),
		networkChannelRollbackTimeout,
	)
	defer cancel()

	for _, sessionID := range createdIDs {
		if strings.TrimSpace(sessionID) == "" {
			continue
		}
		if err := sessions.StopWithCause(
			rollbackCtx,
			sessionID,
			session.CauseFailed,
			"rollback network channel creation",
		); err != nil {
			baseErr = errors.Join(baseErr, err)
		}
	}
	if err := networkStore.DeleteNetworkChannel(rollbackCtx, ref); err != nil {
		baseErr = errors.Join(baseErr, err)
	}
	return baseErr
}
