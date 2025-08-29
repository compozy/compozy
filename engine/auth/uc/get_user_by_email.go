package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/pkg/logger"
)

// GetUserByEmail use case for retrieving a user by email
type GetUserByEmail struct {
	repo  Repository
	email string
}

// NewGetUserByEmail creates a new get user by email use case
func NewGetUserByEmail(repo Repository, email string) *GetUserByEmail {
	return &GetUserByEmail{
		repo:  repo,
		email: email,
	}
}

// Execute retrieves a user by email
func (uc *GetUserByEmail) Execute(ctx context.Context) (*model.User, error) {
	log := logger.FromContext(ctx)
	log.Debug("Getting user by email", "email", uc.email)
	user, err := uc.repo.GetUserByEmail(ctx, uc.email)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by email %s: %w", uc.email, err)
	}
	return user, nil
}
