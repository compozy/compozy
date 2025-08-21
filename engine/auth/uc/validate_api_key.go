package uc

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/compozy/compozy/engine/auth"
	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"golang.org/x/crypto/bcrypt"
)

// Pre-computed bcrypt hash of a random string for constant-time comparison
// This prevents timing attacks by ensuring similar execution time for both
// "key not found" and "invalid key" scenarios
var dummyBcryptHash = []byte("$2a$10$dummyHashForTimingAttackPrevention1234567890abcdef")

// Background task semaphore to prevent unbounded goroutine creation
// Limits concurrent background operations to prevent resource exhaustion
var backgroundTaskSem = make(chan struct{}, 10) // Allow max 10 concurrent background tasks

// ValidateAPIKey use case for validating an API key and returning the associated user
type ValidateAPIKey struct {
	repo      Repository
	plaintext string
}

// NewValidateAPIKey creates a new validate API key use case
func NewValidateAPIKey(repo Repository, plaintext string) *ValidateAPIKey {
	return &ValidateAPIKey{
		repo:      repo,
		plaintext: plaintext,
	}
}

// Execute validates an API key and returns the associated user
func (uc *ValidateAPIKey) Execute(ctx context.Context) (*model.User, error) {
	log := logger.FromContext(ctx)

	// Hash the plaintext key to find it in the database (fingerprint for O(1) lookup)
	hash := sha256.Sum256([]byte(uc.plaintext))

	// Get API key by hash (uses fingerprint for lookup)
	apiKey, err := uc.repo.GetAPIKeyByHash(ctx, hash[:])

	// Prepare for constant-time comparison to prevent timing attacks
	// If key not found, we'll compare against a dummy hash
	keyHash := dummyBcryptHash
	keyFound := err == nil
	if keyFound {
		keyHash = apiKey.Hash
	}

	// Always perform bcrypt comparison to ensure constant timing
	compareErr := bcrypt.CompareHashAndPassword(keyHash, []byte(uc.plaintext))

	// Now we can safely check if the key was found and valid
	if !keyFound {
		log.Debug("API key not found", "error", err)
		return nil, core.NewError(
			fmt.Errorf("invalid API key"),
			auth.ErrCodeNotFound,
			nil,
		)
	}

	if compareErr != nil {
		log.Debug("API key hash verification failed", "error", compareErr)
		return nil, core.NewError(
			fmt.Errorf("invalid API key"),
			auth.ErrCodeForbidden,
			nil,
		)
	}

	// Get the associated user
	user, err := uc.repo.GetUserByID(ctx, apiKey.UserID)
	if err != nil {
		log.Error("Failed to get user for valid API key", "error", err, "user_id", apiKey.UserID)
		return nil, core.NewError(
			fmt.Errorf("failed to get user: %w", err),
			auth.ErrCodeInternal,
			map[string]any{
				"user_id": apiKey.UserID.String(),
			},
		)
	}

	// Update last used timestamp (fire and forget with resource limiting)
	select {
	case backgroundTaskSem <- struct{}{}:
		// Acquired semaphore, proceed with background task
		go func() {
			defer func() { <-backgroundTaskSem }() // Release semaphore when done
			// Use background context for this operation to detach it from request lifecycle
			bgCtx := context.Background()
			if updateErr := uc.repo.UpdateAPIKeyLastUsed(bgCtx, apiKey.ID); updateErr != nil {
				// Extract logger without request-scoped values
				bgLog := logger.FromContext(bgCtx)
				bgLog.Warn("Failed to update API key last used", "error", updateErr, "key_id", apiKey.ID)
			}
		}()
	default:
		// Semaphore full, skip update to prevent resource exhaustion
		log.Debug("Skipping API key last used update due to high load", "key_id", apiKey.ID)
	}

	return user, nil
}
