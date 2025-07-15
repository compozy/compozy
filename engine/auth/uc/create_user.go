package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/auth"
	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/core"
)

// CreateUserInput represents the input for creating a user
type CreateUserInput struct {
	Email string     `json:"email"`
	Role  model.Role `json:"role"`
}

// CreateUser use case for creating a new user
type CreateUser struct {
	repo  Repository
	input *CreateUserInput
}

// NewCreateUser creates a new create user use case
func NewCreateUser(repo Repository, input *CreateUserInput) *CreateUser {
	return &CreateUser{
		repo:  repo,
		input: input,
	}
}

// Execute creates a new user
func (uc *CreateUser) Execute(ctx context.Context) (*model.User, error) {
	// Check if user already exists
	existingUser, err := uc.repo.GetUserByEmail(ctx, uc.input.Email)
	if err == nil && existingUser != nil {
		return nil, core.NewError(
			fmt.Errorf("user already exists"),
			auth.ErrCodeEmailExists,
			map[string]any{
				"email": uc.input.Email,
			},
		)
	}

	// Create user
	user := &model.User{
		ID:    core.MustNewID(),
		Email: uc.input.Email,
		Role:  uc.input.Role,
	}

	// Save user
	if err := uc.repo.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}
