package uc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"golang.org/x/crypto/bcrypt"
)

// GenerateAPIKey use case for generating a new API key for a user
type GenerateAPIKey struct {
	repo   Repository
	userID core.ID
}

// NewGenerateAPIKey creates a new generate API key use case
func NewGenerateAPIKey(repo Repository, userID core.ID) *GenerateAPIKey {
	return &GenerateAPIKey{
		repo:   repo,
		userID: userID,
	}
}

// Execute generates a new API key for the user
func (uc *GenerateAPIKey) Execute(ctx context.Context) (string, error) {
	log := logger.FromContext(ctx)

	// Generate random key part (8 characters)
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	keyPart := make([]byte, 32) // 32 characters for the random part
	for i := range keyPart {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random key part: %w", err)
		}
		keyPart[i] = charset[num.Int64()]
	}

	// Create full key with prefix
	prefix := "cpzy_"
	plaintext := prefix + string(keyPart)

	// Hash the key for storage
	hashedKey, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash API key: %w", err)
	}

	// Create fingerprint for O(1) lookups
	fingerprint := sha256.Sum256([]byte(plaintext))

	// Create API key record
	apiKey := &model.APIKey{
		ID:          core.MustNewID(),
		UserID:      uc.userID,
		Hash:        hashedKey,
		Fingerprint: fingerprint[:],
		Prefix:      prefix,
	}

	// Save to repository
	if err := uc.repo.CreateAPIKey(ctx, apiKey); err != nil {
		log.Error("Failed to create API key", "error", err, "user_id", uc.userID)
		return "", fmt.Errorf("failed to create API key: %w", err)
	}

	log.Info("API key generated successfully", "user_id", uc.userID, "key_id", apiKey.ID)
	return plaintext, nil
}
