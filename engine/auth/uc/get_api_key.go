package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

// GetAPIKey use case for retrieving an API key by ID
type GetAPIKey struct {
	repo  Repository
	keyID core.ID
}

// NewGetAPIKey creates a new get API key use case
func NewGetAPIKey(repo Repository, keyID core.ID) *GetAPIKey {
	return &GetAPIKey{
		repo:  repo,
		keyID: keyID,
	}
}

// Execute retrieves an API key by ID
func (uc *GetAPIKey) Execute(ctx context.Context) (*model.APIKey, error) {
	log := logger.FromContext(ctx)
	log.Debug("Getting API key", "key_id", uc.keyID)
	key, err := uc.repo.GetAPIKeyByID(ctx, uc.keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key %s: %w", uc.keyID, err)
	}
	return key, nil
}
