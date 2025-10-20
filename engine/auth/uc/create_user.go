package uc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
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
	log := logger.FromContext(ctx)
	log.Debug("Creating user", "email", uc.input.Email, "role", uc.input.Role)
	// Check if user already exists
	existingUser, err := uc.repo.GetUserByEmail(ctx, uc.input.Email)
	if err != nil {
		// If user not found, that's expected - we can proceed
		// For any other error, we should fail fast
		if !errors.Is(err, ErrUserNotFound) {
			return nil, fmt.Errorf("checking existing user: %w", err)
		}
	} else if existingUser != nil {
		return nil, ErrEmailExists
	}
	// Generate user ID
	userID, err := core.NewID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate user ID: %w", err)
	}
	// Create user
	user := &model.User{
		ID:        userID,
		Email:     uc.input.Email,
		Role:      uc.input.Role,
		CreatedAt: time.Now().UTC(),
	}
	if err := uc.repo.CreateUser(ctx, user); err != nil {
		if errors.Is(err, ErrEmailExists) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	log.Info("User created successfully", "user_id", user.ID, "email", user.Email, "role", user.Role)
	return user, nil
}
