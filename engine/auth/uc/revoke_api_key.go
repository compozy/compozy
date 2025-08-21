package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/auth"
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
	// Get the API key first
	apiKey, err := uc.repo.GetAPIKeyByID(ctx, uc.keyID)
	if err != nil {
		return core.NewError(
			fmt.Errorf("API key not found: %w", err),
			auth.ErrCodeNotFound,
			map[string]any{
				"key_id": uc.keyID.String(),
			},
		)
	}
	// Check if the key belongs to the requesting user
	if apiKey.UserID != uc.userID {
		return core.NewError(
			fmt.Errorf("access denied"),
			auth.ErrCodeForbidden,
			map[string]any{
				"key_id": uc.keyID.String(),
			},
		)
	}
	// Delete the key
	if err := uc.repo.DeleteAPIKey(ctx, uc.keyID); err != nil {
		return core.NewError(
			fmt.Errorf("failed to revoke API key: %w", err),
			auth.ErrCodeInternal,
			map[string]any{
				"key_id": uc.keyID.String(),
			},
		)
	}
	log.Info("API key revoked successfully", "key_id", uc.keyID, "user_id", uc.userID)
	return nil
}
