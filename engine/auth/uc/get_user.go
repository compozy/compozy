package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

// GetUser use case for retrieving a user by ID
type GetUser struct {
	repo   Repository
	userID core.ID
}

// NewGetUser creates a new get user use case
func NewGetUser(repo Repository, userID core.ID) *GetUser {
	return &GetUser{
		repo:   repo,
		userID: userID,
	}
}

// Execute retrieves a user by ID
func (uc *GetUser) Execute(ctx context.Context) (*model.User, error) {
	log := logger.FromContext(ctx)
	log.Debug("Getting user", "user_id", uc.userID)
	user, err := uc.repo.GetUserByID(ctx, uc.userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user %s: %w", uc.userID, err)
	}
	return user, nil
}
