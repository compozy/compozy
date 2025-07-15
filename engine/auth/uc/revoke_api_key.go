package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/auth"
	"github.com/compozy/compozy/engine/core"
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
	// First, get the API key to verify ownership
	apiKey, err := uc.repo.GetAPIKeyByID(ctx, uc.keyID)
	if err != nil {
		return core.NewError(
			fmt.Errorf("API key not found"),
			auth.ErrCodeNotFound,
			map[string]any{
				"key_id": uc.keyID,
			},
		)
	}

	// Verify the key belongs to the user
	if apiKey.UserID != uc.userID {
		return core.NewError(
			fmt.Errorf("access denied"),
			auth.ErrCodeForbidden,
			map[string]any{
				"key_id": uc.keyID,
			},
		)
	}

	// Now revoke the key
	if err := uc.repo.DeleteAPIKey(ctx, uc.keyID); err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}
	return nil
}
