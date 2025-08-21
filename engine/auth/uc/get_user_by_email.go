package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/auth"
	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/core"
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
	user, err := uc.repo.GetUserByEmail(ctx, uc.email)
	if err != nil {
		return nil, core.NewError(
			fmt.Errorf("failed to get user by email: %w", err),
			auth.ErrCodeNotFound,
			map[string]any{
				"email": uc.email,
			},
		)
	}
	return user, nil
}
