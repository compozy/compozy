package uc

import (
	"context"
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
	if err == nil && existingUser != nil {
		return nil, fmt.Errorf("user already exists with email %s", uc.input.Email)
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
		return nil, fmt.Errorf("failed to create user with email %s: %w", uc.input.Email, err)
	}
	log.Info("User created successfully", "user_id", user.ID, "email", user.Email, "role", user.Role)
	return user, nil
}
