package uc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

// RevokeAPIKey use case for revoking (deleting) an API key
type RevokeAPIKey struct {
	repo   Repository
	userID core.ID
	keyID  core.ID
}

// NewRevokeAPIKey creates a new revoke API key use case
func NewRevokeAPIKey(repo Repository, userID, keyID core.ID) *RevokeAPIKey {
	return &RevokeAPIKey{
		repo:   repo,
		userID: userID,
		keyID:  keyID,
	}
}

// Execute revokes an API key after verifying ownership
func (uc *RevokeAPIKey) Execute(ctx context.Context) error {
	log := logger.FromContext(ctx)
	log.Debug("Revoking API key", "key_id", uc.keyID)
	apiKey, err := uc.repo.GetAPIKeyByID(ctx, uc.keyID)
	if err != nil {
		if errors.Is(err, ErrAPIKeyNotFound) {
			return err
		}
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			return fmt.Errorf("%w: %w", ErrAPIKeyNotFound, err)
		}
		return fmt.Errorf("failed to retrieve API key %s: %w", uc.keyID, err)
	}
	if apiKey.UserID != uc.userID {
		return fmt.Errorf("access denied to API key %s", uc.keyID)
	}
	if err := uc.repo.DeleteAPIKey(ctx, uc.keyID); err != nil {
		return fmt.Errorf("failed to revoke API key %s: %w", uc.keyID, err)
	}
	log.Info("API key revoked successfully", "key_id", uc.keyID, "user_id", uc.userID)
	return nil
}
