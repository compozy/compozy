package uc

import (
	"context"
	"errors"
	"fmt"

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
	if err := uc.repo.DeleteUser(ctx, uc.userID); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return err
		}
		return fmt.Errorf("failed to delete user %s: %w", uc.userID, err)
	}
	log.Info("User deleted successfully", "user_id", uc.userID)
	return nil
}
