package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/auth"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

// DeleteUser use case for deleting a user
type DeleteUser struct {
	repo   Repository
	userID core.ID
}

// NewDeleteUser creates a new delete user use case
func NewDeleteUser(repo Repository, userID core.ID) *DeleteUser {
	return &DeleteUser{
		repo:   repo,
		userID: userID,
	}
}

// Execute deletes a user
func (uc *DeleteUser) Execute(ctx context.Context) error {
	log := logger.FromContext(ctx)
	log.Debug("Deleting user", "user_id", uc.userID)
	// Check if user exists
	_, err := uc.repo.GetUserByID(ctx, uc.userID)
	if err != nil {
		return core.NewError(
			fmt.Errorf("user not found: %w", err),
			auth.ErrCodeNotFound,
			map[string]any{
				"user_id": uc.userID.String(),
			},
		)
	}
	// Delete user
	if err := uc.repo.DeleteUser(ctx, uc.userID); err != nil {
		return core.NewError(
			fmt.Errorf("failed to delete user: %w", err),
			auth.ErrCodeInternal,
			map[string]any{
				"user_id": uc.userID.String(),
			},
		)
	}
	log.Info("User deleted successfully", "user_id", uc.userID)
	return nil
}
