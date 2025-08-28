package uc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/core"
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
	// Generate user ID
	userID, err := core.NewID()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate user ID: %w", err)
	}
	// Create admin user atomically
	user := &model.User{
		ID:        userID,
		Email:     uc.email,
		Role:      model.RoleAdmin,
		CreatedAt: time.Now().UTC(),
	}
	// Use atomic operation to prevent race condition
	if err := uc.repo.CreateInitialAdminIfNone(ctx, user); err != nil {
		// Check if it's an already-bootstrapped error
		var coreErr *core.Error
		if errors.As(err, &coreErr) && coreErr.Code == "ALREADY_BOOTSTRAPPED" {
			return nil, "", fmt.Errorf("system already bootstrapped")
		}
		return nil, "", fmt.Errorf("creating admin user: %w", err)
	}
	// Generate API key for the new admin
	genUC := NewGenerateAPIKey(uc.repo, user.ID)
	apiKey, err := genUC.Execute(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("generating API key: %w", err)
	}
	return user, apiKey, nil
}
