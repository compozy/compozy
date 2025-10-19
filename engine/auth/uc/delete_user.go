package uc

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
		if errors.Is(err, ErrUserNotFound) {
			return err
		}
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			return fmt.Errorf("%w: %w", ErrUserNotFound, err)
		}
		return fmt.Errorf("failed to retrieve user %s: %w", uc.userID, err)
	}
	// Delete the user
	if err := uc.repo.DeleteUser(ctx, uc.userID); err != nil {
		return fmt.Errorf("failed to delete user %s: %w", uc.userID, err)
	}
	log.Info("User deleted successfully", "user_id", uc.userID)
	return nil
}
