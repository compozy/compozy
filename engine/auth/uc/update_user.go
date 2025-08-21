package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/auth"
	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

// UpdateUserInput represents the input for updating a user
type UpdateUserInput struct {
	Email *string     `json:"email,omitempty"`
	Role  *model.Role `json:"role,omitempty"`
}

// UpdateUser use case for updating a user
type UpdateUser struct {
	repo   Repository
	userID core.ID
	input  *UpdateUserInput
}

// NewUpdateUser creates a new update user use case
func NewUpdateUser(repo Repository, userID core.ID, input *UpdateUserInput) *UpdateUser {
	return &UpdateUser{
		repo:   repo,
		userID: userID,
		input:  input,
	}
}

// Execute updates a user
func (uc *UpdateUser) Execute(ctx context.Context) (*model.User, error) {
	log := logger.FromContext(ctx)
	log.Debug("Updating user", "user_id", uc.userID)
	// Get existing user
	user, err := uc.repo.GetUserByID(ctx, uc.userID)
	if err != nil {
		return nil, core.NewError(
			fmt.Errorf("user not found: %w", err),
			auth.ErrCodeNotFound,
			map[string]any{
				"user_id": uc.userID.String(),
			},
		)
	}
	// Update fields
	if uc.input.Email != nil {
		// Check if new email already exists
		existingUser, err := uc.repo.GetUserByEmail(ctx, *uc.input.Email)
		if err == nil && existingUser != nil && existingUser.ID != uc.userID {
			return nil, core.NewError(
				fmt.Errorf("email already exists"),
				auth.ErrCodeEmailExists,
				map[string]any{
					"email": *uc.input.Email,
				},
			)
		}
		user.Email = *uc.input.Email
	}
	if uc.input.Role != nil {
		user.Role = *uc.input.Role
	}
	// Update in repository
	if err := uc.repo.UpdateUser(ctx, user); err != nil {
		return nil, core.NewError(
			fmt.Errorf("failed to update user: %w", err),
			auth.ErrCodeInternal,
			map[string]any{
				"user_id": uc.userID.String(),
			},
		)
	}
	log.Info("User updated successfully", "user_id", user.ID)
	return user, nil
}
