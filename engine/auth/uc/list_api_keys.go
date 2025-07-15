package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/core"
)

// ListAPIKeys use case for listing all API keys for a user
type ListAPIKeys struct {
	repo   Repository
	userID core.ID
}

// NewListAPIKeys creates a new list API keys use case
func NewListAPIKeys(repo Repository, userID core.ID) *ListAPIKeys {
	return &ListAPIKeys{
		repo:   repo,
		userID: userID,
	}
}

// Execute lists all API keys for a user
func (uc *ListAPIKeys) Execute(ctx context.Context) ([]*model.APIKey, error) {
	apiKeys, err := uc.repo.ListAPIKeysByUserID(ctx, uc.userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}
	return apiKeys, nil
}
