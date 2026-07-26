package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

const catalogIdentityInvalidationTimeout = 5 * time.Second

func (s *Store) invalidateCatalogIdentityAfterSyncFailure(
	ctx context.Context,
	scope memcontract.Scope,
	action string,
	filename string,
	syncErr error,
) error {
	if syncErr == nil || s.catalog == nil {
		return syncErr
	}
	cleanupCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		catalogIdentityInvalidationTimeout,
	)
	defer cancel()

	_, workspaceID, err := s.catalogWorkspaceForScope(cleanupCtx, scope)
	if err == nil {
		err = s.catalog.invalidateCatalogIdentity(cleanupCtx, newCatalogIdentity(
			scope,
			workspaceID,
			s.catalogAgentName(scope),
			s.catalogAgentTier(scope),
		))
	}
	if err == nil {
		return syncErr
	}

	invalidationErr := fmt.Errorf("memory: invalidate catalog identity after %s: %w", action, err)
	s.warn(
		"memory: catalog identity invalidation failed after derived sync failure",
		"action", action,
		"scope", scope.Normalize(),
		"filename", strings.TrimSpace(filename),
		"sync_error", syncErr,
		"invalidation_error", invalidationErr,
	)
	return errors.Join(syncErr, invalidationErr)
}
