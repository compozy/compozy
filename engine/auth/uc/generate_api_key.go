package uc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"golang.org/x/crypto/bcrypt"
)

const (
	apiKeyPrefix      = "cpzy"
	apiKeyTokenPrefix = "cpzy_"
	keyPreviewLen     = 8 // Number of characters to preview after the prefix
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
	log.Debug("Generating API key for user", "user_id", uc.userID)
	// Generate a cryptographically secure random key
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random key part: %w", err)
	}
	// Create the plaintext API key
	plaintext := fmt.Sprintf("%s_%s", apiKeyPrefix, base64.RawURLEncoding.EncodeToString(randomBytes))
	// Hash the key for storage
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash API key: %w", err)
	}
	// Generate a SHA256 fingerprint for O(1) lookups
	fingerprintHash := sha256.Sum256([]byte(plaintext))
	// Store the key
	apiKey := &model.APIKey{
		ID:          core.MustNewID(),
		UserID:      uc.userID,
		Prefix:      plaintext[:len(apiKeyTokenPrefix)+keyPreviewLen], // e.g., "cpzy_"+first 8 chars
		Hash:        hashedBytes,
		Fingerprint: fingerprintHash[:],
		CreatedAt:   time.Now().UTC(),
	}
	if err := uc.repo.CreateAPIKey(ctx, apiKey); err != nil {
		log.Error("Failed to create API key", "error", err, "user_id", uc.userID)
		return "", fmt.Errorf("failed to create API key for user %s: %w", uc.userID, err)
	}
	log.Info("API key generated successfully", "key_id", apiKey.ID, "user_id", uc.userID, "prefix", apiKey.Prefix)
	return plaintext, nil
}
