package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/auth/model"
)

// BootstrapSystem is a one-time use-case that initializes the very first
// admin user (and API key) directly in the database, bypassing any HTTP layer.
type BootstrapSystem struct {
	repo  Repository
	email string
}

// NewBootstrapSystem constructs the use-case.
func NewBootstrapSystem(repo Repository, email string) *BootstrapSystem {
	return &BootstrapSystem{
		repo:  repo,
		email: email,
	}
}

// Execute creates the first admin user and API key.
// It returns the created user and the plaintext API key.
func (uc *BootstrapSystem) Execute(ctx context.Context) (*model.User, string, error) {
	// 1. Make sure we are not already bootstrapped.
	users, err := uc.repo.ListUsers(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("listing users: %w", err)
	}
	for _, u := range users {
		if u.Role == model.RoleAdmin {
			return nil, "", fmt.Errorf("system already bootstrapped")
		}
	}

	// 2. Create admin user.
	createInput := &CreateUserInput{
		Email: uc.email,
		Role:  model.RoleAdmin,
	}
	createUC := NewCreateUser(uc.repo, createInput)

	user, err := createUC.Execute(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("creating admin user: %w", err)
	}

	// 3. Generate API key for the new admin.
	genUC := NewGenerateAPIKey(uc.repo, user.ID)
	apiKey, err := genUC.Execute(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("generating API key: %w", err)
	}

	return user, apiKey, nil
}
