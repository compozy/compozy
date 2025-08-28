package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/auth/model"
)

// ListUsers use case for retrieving all users
type ListUsers struct {
	repo Repository
}

// NewListUsers creates a new list users use case
func NewListUsers(repo Repository) *ListUsers {
	return &ListUsers{
		repo: repo,
	}
}

// Execute retrieves all users
func (uc *ListUsers) Execute(ctx context.Context) ([]*model.User, error) {
	users, err := uc.repo.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	return users, nil
}
