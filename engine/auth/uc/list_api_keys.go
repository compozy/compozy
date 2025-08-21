package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/auth"
	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
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
	log := logger.FromContext(ctx)
	log.Debug("Listing API keys for user", "user_id", uc.userID)
	keys, err := uc.repo.ListAPIKeysByUserID(ctx, uc.userID)
	if err != nil {
		return nil, core.NewError(
			fmt.Errorf("failed to list API keys: %w", err),
			auth.ErrCodeInternal,
			map[string]any{
				"user_id": uc.userID.String(),
			},
		)
	}
	return keys, nil
}
