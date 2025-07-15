package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/core"
)

// GetAPIKey use case for retrieving an API key by ID
type GetAPIKey struct {
	repo Repository
}

// NewGetAPIKey creates a new get API key use case
func NewGetAPIKey(repo Repository) *GetAPIKey {
	return &GetAPIKey{
		repo: repo,
	}
}

// Execute retrieves an API key by ID
func (uc *GetAPIKey) Execute(ctx context.Context, keyID core.ID) (*model.APIKey, error) {
	apiKey, err := uc.repo.GetAPIKeyByID(ctx, keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}
	return apiKey, nil
}
