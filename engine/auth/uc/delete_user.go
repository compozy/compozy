package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/auth"
	"github.com/compozy/compozy/engine/core"
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
	// First check if user exists
	_, err := uc.repo.GetUserByID(ctx, uc.userID)
	if err != nil {
		return core.NewError(
			fmt.Errorf("user not found"),
			auth.ErrCodeNotFound,
			map[string]any{
				"user_id": uc.userID,
			},
		)
	}

	if err := uc.repo.DeleteUser(ctx, uc.userID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	return nil
}
